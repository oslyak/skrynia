# Як підключити `skrynia` до власного Go-проекту

Інструкція для розробників, які хочуть використовувати пакети `vault` та `tpmkey`
як бібліотеки в своїх Go-програмах.

> English version: [library-integration.md](./library-integration.md)

## 1. Додати залежність

```bash
cd your-project
go get github.com/oslyak/skrynia@latest
```

У `go.mod` з'явиться:

```go
require github.com/oslyak/skrynia v1.0.9
```

Можна імпортувати **обидва** пакети або **тільки один** — вони розділені:

- `github.com/oslyak/skrynia/vault` — високорівневе сховище (AES-256-GCM JSON,
  get/set/list/delete/env/export/import). Більшість проектів хоче саме це.
- `github.com/oslyak/skrynia/tpmkey` — низькорівневий TPM seal/unseal
  32-байтного ключа (без JSON-сховища зверху).

**Важливо:** обидва пакети **НЕ тягнуть GTK4**. GUI живе в `cmd/skrynia/` і не
експортується як бібліотека.

## 2. Вимоги до середовища

| Платформа | Що треба                                                   |
|-----------|------------------------------------------------------------|
| Linux     | TPM 2.0 device `/dev/tpmrm0`, користувач у групі `tss`     |
| Windows   | TPM 2.0 + TBS API (без особливих прав)                     |
| macOS     | TPM відсутній → бібліотека поверне помилку                 |

Перевірка перед роботою:

```go
if !tpmkey.Available() {
    log.Fatal("TPM 2.0 required")
}
```

## 3. Типовий сценарій — vault

```go
package main

import (
    "fmt"
    "log"

    "github.com/oslyak/skrynia/vault"
)

func main() {
    // Варіант А: стандартне місце
    //   ~/.local/share/skrynia/vault.{key,dat} на Linux
    //   %APPDATA%\skrynia\vault.{key,dat}      на Windows
    path, err := vault.DefaultPath()
    if err != nil {
        log.Fatal(err)
    }

    // Варіант Б: власний шлях (без розширення — vault сам додасть .key і .dat)
    // path := "/var/lib/myapp/secrets"

    v, err := vault.Open(path)
    if err != nil {
        log.Fatal(err) // TPM недоступний, немає доступу до /dev/tpmrm0, пошкоджений vault тощо
    }
    defer v.Close() // ⚠️ обов'язково: зберігає + занулює ключ у пам'яті

    // Читання
    token, err := v.Get("github", "token")
    if err == vault.ErrNotFound {
        fmt.Println("token not set")
    }

    // Запис
    if err := v.Set("github", "token", "ghp_xxx..."); err != nil {
        log.Fatal(err)
    }

    // Перелік
    services, _ := v.List("")           // ["github", "redmine"]
    keys, _     := v.List("github")     // ["login", "token"]

    // Масовий експорт у env-style
    env, _ := v.Env("github")           // map["LOGIN":..., "TOKEN":...] (UPPER, - → _)
    for k, val := range env {
        fmt.Printf("%s=%s\n", k, val)
    }

    // Бекап (зашифрований блоб з magic "SKR1")
    blob, _ := v.Export()
    // ... зберегти blob ...
    // v.Import(blob) — відновити
}
```

## 4. Низькорівневе використання — tpmkey

Коли **не** потрібен JSON-store, а лише TPM-прив'язаний ключ для власного крипто:

```go
package main

import "github.com/oslyak/skrynia/tpmkey"

func main() {
    // Перший запуск: згенерувати 32-байтний ключ + запечатати під SRK
    blob, key, err := tpmkey.SealNewKeyRetain()
    if err != nil {
        panic(err)
    }
    defer zero(key) // занулити після використання
    // ... зберегти blob у файл ...
    _ = blob

    // Наступні запуски: розпечатати
    // blob, _ := os.ReadFile("myapp.key")
    // key, err := tpmkey.Unseal(blob)

    // Далі key (32 байти) можна використовувати напряму як AES-256 ключ,
    // HKDF seed, HMAC key тощо.
    _ = key
}

func zero(b []byte) {
    for i := range b {
        b[i] = 0
    }
}
```

## 5. Конкурентність

`*vault.Vault` **безпечний для конкурентного використання** всередині одного
процесу (внутрішній `sync.Mutex`). Усі операції `Get/Set/List/Delete/Env/Export/Import`
серіалізуються.

**Але**: один файл vault не розрахований на кілька процесів одночасно — запис
з двох процесів дасть гонку на `.dat`. Якщо кілька процесів потребують одного
сховища, запустіть окремий процес-власник, до якого вони звертаються через IPC
(або через CLI `skrynia get ...`).

## 6. Обробка помилок

Експортовані sentinel-помилки:

```go
vault.ErrNotFound      // сервіс або ключ не існує
vault.ErrBadMagic      // Import отримав блоб без "SKR1" префіксу
vault.ErrBadPayload    // Import не зміг розшифрувати/розпарсити payload
```

Перевірка через `errors.Is`:

```go
if _, err := v.Get("svc", "key"); errors.Is(err, vault.ErrNotFound) {
    // ...
}
```

## 7. Збірка вашого проекту

- **Linux**: `CGO_ENABLED=0` — бібліотеки `vault` і `tpmkey` **не** потребують
  CGO. Чистий Go, статичне лінкування працює.
- **Windows**: те саме — чистий syscall.
- Якщо ваш бінарник сам використовує CGO з інших причин — все одно сумісно.

```bash
CGO_ENABLED=0 go build -o myapp ./cmd/myapp
```

## 8. Обмеження та вимоги зберігання

1. **Ключ прив'язаний до заліза.** `vault.dat` + `vault.key` з машини А
   **неможливо** розшифрувати на машині Б, навіть на тій самій ОС з тим самим
   користувачем. Це by design — фічa, а не баг. Для міграції використовуйте
   `Export()` → перенесення → `Import()`.

2. **Видалення `vault.key` = втрата даних.** Відновлення немає.

3. **Linux без групи `tss`**: `vault.Open` поверне помилку
   `TPM 2.0 not available`. Рішення: `sudo usermod -aG tss $USER` + перелогін,
   або запуск через `sg tss -c "...".`

4. **Docker/контейнери**: прокиньте `/dev/tpmrm0` всередину контейнера
   (`--device=/dev/tpmrm0`), інакше TPM недоступний. Для dev-середовищ без TPM
   зазвичай простіше використовувати CLI `skrynia get ...` на хості.

5. **Не забувайте `defer v.Close()`** — без нього остання зміна може не
   записатись, а майстер-ключ лишиться у пам'яті процесу.

## 9. Мінімальний приклад `go.mod`

```go
module example.com/myapp

go 1.25

require github.com/oslyak/skrynia v1.0.9
```

Це все — далі працюєте з `vault.Open()` як з будь-яким локальним сховищем.

## Публічне API — стислий довідник

### Пакет `vault`

| Символ                                           | Опис                                                   |
|--------------------------------------------------|--------------------------------------------------------|
| `DefaultPath() (string, error)`                  | Стандартне місце за платформою                         |
| `Open(basePath string) (*Vault, error)`          | Відкрити/створити сховище                              |
| `(*Vault).Close() error`                         | Зберегти + занулити ключ                               |
| `(*Vault).Get(service, key) (string, error)`     | Прочитати значення                                     |
| `(*Vault).Set(service, key, value) error`        | Записати значення                                      |
| `(*Vault).List(service) ([]string, error)`       | `""` → усі сервіси; інакше — ключі сервісу             |
| `(*Vault).Delete(service, key) error`            | `key=""` → видалити весь сервіс                        |
| `(*Vault).Env(service) (map[string]string, error)` | KEY=VALUE з нормалізацією (UPPER, `-` → `_`)         |
| `(*Vault).Export() ([]byte, error)`              | Зашифрований блоб `SKR1`                               |
| `(*Vault).Import(blob []byte) error`             | Merge з блобу                                          |
| `ErrNotFound`, `ErrBadMagic`, `ErrBadPayload`    | Sentinel-помилки                                       |

### Пакет `tpmkey`

| Символ                                           | Опис                                                   |
|--------------------------------------------------|--------------------------------------------------------|
| `Available() bool`                               | Чи доступний TPM (без помилки, просто true/false)      |
| `SealNewKey() ([]byte, error)`                   | Згенерувати+запечатати ключ, повернути лише блоб       |
| `SealNewKeyRetain() ([]byte, []byte, error)`     | Те саме + повернути plaintext ключ (caller занулює)    |
| `Unseal(blob []byte) ([]byte, error)`            | Розпечатати ключ назад                                 |
