//go:build !windows

package main

import (
	_ "embed"
	"fmt"
	"os"

	"github.com/diamondburned/gotk4/pkg/gdk/v4"
	"github.com/diamondburned/gotk4/pkg/gio/v2"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"

	"skrynia/vault"
)

//go:embed icon.png
var iconData []byte

func runGUI(v *vault.Vault, service, template, lang string) {
	app := gtk.NewApplication("com.skrynia.dialog", gio.ApplicationDefaultFlags)

	app.ConnectActivate(func() {
		switch template {
		case "credentials":
			showCredentialsWindow(app, v, service, lang)
		case "api-key":
			showAPIKeyWindow(app, v, service, lang)
		default:
			showSingleFieldWindow(app, v, service, template, lang)
		}
	})

	app.Run(nil)
}

func makeTitle(lang, suffix string) string {
	if lang == "uk" {
		return fmt.Sprintf("Скриня v%s — %s", version, suffix)
	}
	return fmt.Sprintf("Skrynia v%s — %s", version, suffix)
}

func btnText(lang string) string {
	if lang == "uk" {
		return "Зберегти"
	}
	return "Save"
}

func showCredentialsWindow(app *gtk.Application, v *vault.Vault, service, lang string) {
	win := gtk.NewApplicationWindow(app)
	win.SetTitle(makeTitle(lang, service))
	win.SetDefaultSize(420, 200)
	win.SetResizable(false)

	box := gtk.NewBox(gtk.OrientationVertical, 8)
	box.SetMarginTop(15)
	box.SetMarginBottom(15)
	box.SetMarginStart(15)
	box.SetMarginEnd(15)

	loginLabel := gtk.NewLabel("Login")
	if lang == "uk" {
		loginLabel.SetText("Логін")
	}
	loginLabel.SetHAlign(gtk.AlignStart)
	loginEntry := gtk.NewEntry()
	if val, err := v.Get(service, "login"); err == nil {
		loginEntry.SetText(val)
	}

	passLabel := gtk.NewLabel("Password")
	if lang == "uk" {
		passLabel.SetText("Пароль")
	}
	passLabel.SetHAlign(gtk.AlignStart)
	passEntry := gtk.NewPasswordEntry()
	passEntry.SetShowPeekIcon(true)
	if val, err := v.Get(service, "password"); err == nil {
		passEntry.SetText(val)
	}

	saveBtn := gtk.NewButtonWithLabel(btnText(lang))
	saveBtn.AddCSSClass("suggested-action")
	saveBtn.SetHAlign(gtk.AlignEnd)

	saveBtn.ConnectClicked(func() {
		login := loginEntry.Text()
		password := passEntry.Text()
		if err := v.Set(service, "login", login); err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			return
		}
		if err := v.Set(service, "password", password); err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			return
		}
		win.Close()
	})

	connectEnterToSave(loginEntry, saveBtn)
	connectPasswordEnterToSave(passEntry, saveBtn)

	box.Append(loginLabel)
	box.Append(loginEntry)
	box.Append(passLabel)
	box.Append(passEntry)
	box.Append(saveBtn)

	win.SetChild(box)
	addEscHandler(win)
	win.Present()
}

func showAPIKeyWindow(app *gtk.Application, v *vault.Vault, service, lang string) {
	win := gtk.NewApplicationWindow(app)
	win.SetTitle(makeTitle(lang, service+" — API Key"))
	win.SetDefaultSize(420, 150)
	win.SetResizable(false)

	box := gtk.NewBox(gtk.OrientationVertical, 8)
	box.SetMarginTop(15)
	box.SetMarginBottom(15)
	box.SetMarginStart(15)
	box.SetMarginEnd(15)

	keyLabel := gtk.NewLabel("API Key")
	if lang == "uk" {
		keyLabel.SetText("API Ключ")
	}
	keyLabel.SetHAlign(gtk.AlignStart)
	keyEntry := gtk.NewPasswordEntry()
	keyEntry.SetShowPeekIcon(true)
	if val, err := v.Get(service, "api-key"); err == nil {
		keyEntry.SetText(val)
	}

	saveBtn := gtk.NewButtonWithLabel(btnText(lang))
	saveBtn.AddCSSClass("suggested-action")
	saveBtn.SetHAlign(gtk.AlignEnd)

	saveBtn.ConnectClicked(func() {
		if err := v.Set(service, "api-key", keyEntry.Text()); err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			return
		}
		win.Close()
	})

	connectPasswordEnterToSave(keyEntry, saveBtn)

	box.Append(keyLabel)
	box.Append(keyEntry)
	box.Append(saveBtn)

	win.SetChild(box)
	addEscHandler(win)
	win.Present()
}

func showSingleFieldWindow(app *gtk.Application, v *vault.Vault, service, key, lang string) {
	win := gtk.NewApplicationWindow(app)
	win.SetTitle(makeTitle(lang, service+" — "+key))
	win.SetDefaultSize(420, 150)
	win.SetResizable(false)

	box := gtk.NewBox(gtk.OrientationVertical, 8)
	box.SetMarginTop(15)
	box.SetMarginBottom(15)
	box.SetMarginStart(15)
	box.SetMarginEnd(15)

	label := gtk.NewLabel(key)
	label.SetHAlign(gtk.AlignStart)
	entry := gtk.NewEntry()
	if val, err := v.Get(service, key); err == nil {
		entry.SetText(val)
	}

	saveBtn := gtk.NewButtonWithLabel(btnText(lang))
	saveBtn.AddCSSClass("suggested-action")
	saveBtn.SetHAlign(gtk.AlignEnd)

	saveBtn.ConnectClicked(func() {
		if err := v.Set(service, key, entry.Text()); err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			return
		}
		win.Close()
	})

	connectEnterToSave(entry, saveBtn)

	box.Append(label)
	box.Append(entry)
	box.Append(saveBtn)

	win.SetChild(box)
	addEscHandler(win)
	win.Present()
}

func addEscHandler(win *gtk.ApplicationWindow) {
	keyCtrl := gtk.NewEventControllerKey()
	keyCtrl.ConnectKeyPressed(func(keyval, keycode uint, state gdk.ModifierType) bool {
		if keyval == 0xff1b { // GDK_KEY_Escape
			win.Close()
			return true
		}
		return false
	})
	win.AddController(keyCtrl)
}

// connectEnterToSave connects Enter key on entry to trigger save button.
func connectEnterToSave(entry *gtk.Entry, saveBtn *gtk.Button) {
	entry.ConnectActivate(func() {
		saveBtn.Activate()
	})
}

// connectPasswordEnterToSave connects Enter key on password entry to trigger save button.
func connectPasswordEnterToSave(entry *gtk.PasswordEntry, saveBtn *gtk.Button) {
	entry.ConnectActivate(func() {
		saveBtn.Activate()
	})
}
