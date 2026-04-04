//go:build windows

package main

import (
	_ "embed"
	"fmt"
	"os"
	"runtime"
	"syscall"
	"unsafe"

	"skrynia/vault"
)

//go:embed icon.ico
var iconData []byte

var (
	user32   = syscall.NewLazyDLL("user32.dll")
	kernel32 = syscall.NewLazyDLL("kernel32.dll")
	gdi32    = syscall.NewLazyDLL("gdi32.dll")

	registerClassExW    = user32.NewProc("RegisterClassExW")
	createWindowExW     = user32.NewProc("CreateWindowExW")
	showWindow          = user32.NewProc("ShowWindow")
	updateWindow        = user32.NewProc("UpdateWindow")
	getMessage          = user32.NewProc("GetMessageW")
	translateMessage    = user32.NewProc("TranslateMessage")
	dispatchMessage     = user32.NewProc("DispatchMessageW")
	isDialogMessage     = user32.NewProc("IsDialogMessageW")
	defWindowProc       = user32.NewProc("DefWindowProcW")
	postQuitMessage     = user32.NewProc("PostQuitMessage")
	destroyWindow       = user32.NewProc("DestroyWindow")
	sendMessage         = user32.NewProc("SendMessageW")
	getWindowTextW      = user32.NewProc("GetWindowTextW")
	getWindowTextLength = user32.NewProc("GetWindowTextLengthW")
	setFocus            = user32.NewProc("SetFocus")
	getModuleHandle     = kernel32.NewProc("GetModuleHandleW")
	loadCursor          = user32.NewProc("LoadCursorW")
	createIconFromResourceEx = user32.NewProc("CreateIconFromResourceEx")
	getSystemMetrics    = user32.NewProc("GetSystemMetrics")
	setWindowPos        = user32.NewProc("SetWindowPos")
	getSysColorBrush    = user32.NewProc("GetSysColorBrush")
	getWindowLong       = user32.NewProc("GetWindowLongPtrW")
	setWindowLong       = user32.NewProc("SetWindowLongPtrW")
	invalidateRect      = user32.NewProc("InvalidateRect")
)

const (
	wsOverlapped   = 0x00000000
	wsCaption      = 0x00C00000
	wsSysMenu      = 0x00080000
	wsMinimizeBox  = 0x00020000
	wsVisible      = 0x10000000
	wsChild        = 0x40000000
	wsTabStop      = 0x00010000
	wsBorder       = 0x00800000
	wsGroup        = 0x00020000
	esPassword     = 0x00020
	esAutoHScroll  = 0x00080
	bsPushButton    = 0x00000000
	bsDefPushButton = 0x00000001
	ssLeft         = 0x00000000
	wmCreate       = 0x0001
	wmDestroy      = 0x0002
	wmCommand      = 0x0111
	wmSetFont      = 0x0030
	wmSetIcon      = 0x0080
	wmClose        = 0x0010
	bnClicked      = 0
	swShow         = 5
	smCxScreen     = 0
	smCyScreen     = 1
	colorBtnFace   = 15
	iconSmall      = 0
	iconBig        = 1
	imageCursor    = 2
	idrArrow       = 32512
	lrDefaultSize  = 0x00000040
)

type wndClassExW struct {
	size       uint32
	style      uint32
	wndProc    uintptr
	clsExtra   int32
	wndExtra   int32
	instance   syscall.Handle
	icon       syscall.Handle
	cursor     syscall.Handle
	background syscall.Handle
	menuName   *uint16
	className  *uint16
	iconSm     syscall.Handle
}

type msg struct {
	hwnd    syscall.Handle
	message uint32
	wParam  uintptr
	lParam  uintptr
	time    uint32
	pt      point
}

type point struct {
	x, y int32
}

// dialog state
type dialogState struct {
	hwndEdits    []syscall.Handle
	pwdEdits     map[syscall.Handle]bool // password edits: handle → currently masked
	saved        bool
	results      []string
}

var dlgState dialogState

const (
	idSaveBtn   = 1001
	idFirstEdit = 2000
	idFirstEye  = 3000
	gwlStyle    = -16
	emSetPasswordChar = 0x00CC
)

func runGUI(v *vault.Vault, service, template, lang string) {
	switch template {
	case "credentials":
		runWin32Dialog(v, service, lang, credentialsFields(v, service, lang))
	case "api-key":
		runWin32Dialog(v, service, lang, apiKeyFields(v, service, lang))
	default:
		runWin32Dialog(v, service, lang, singleFieldFields(v, service, template, lang))
	}
}

type field struct {
	label    string
	value    string
	password bool
	key      string // vault key to save to
}

type dialogDef struct {
	title  string
	fields []field
	btnLabel string
}

func credentialsFields(v *vault.Vault, service, lang string) dialogDef {
	title := fmt.Sprintf("Скриня v%s — %s", version, service)
	loginLabel, passLabel, btnLabel := "Логін", "Пароль", "Зберегти"
	if lang == "en" {
		title = fmt.Sprintf("Skrynia v%s — %s", version, service)
		loginLabel, passLabel, btnLabel = "Login", "Password", "Save"
	}
	existingLogin := ""
	if val, err := v.Get(service, "login"); err == nil {
		existingLogin = val
	}
	existingPass := ""
	if val, err := v.Get(service, "password"); err == nil {
		existingPass = val
	}
	return dialogDef{
		title: title,
		btnLabel: btnLabel,
		fields: []field{
			{label: loginLabel, value: existingLogin, key: "login"},
			{label: passLabel, value: existingPass, password: true, key: "password"},
		},
	}
}

func apiKeyFields(v *vault.Vault, service, lang string) dialogDef {
	title := fmt.Sprintf("Скриня v%s — %s — API Key", version, service)
	keyLabel, btnLabel := "API Ключ", "Зберегти"
	if lang == "en" {
		title = fmt.Sprintf("Skrynia v%s — %s — API Key", version, service)
		keyLabel, btnLabel = "API Key", "Save"
	}
	existingKey := ""
	if val, err := v.Get(service, "api-key"); err == nil {
		existingKey = val
	}
	return dialogDef{
		title: title,
		btnLabel: btnLabel,
		fields: []field{
			{label: keyLabel, value: existingKey, password: true, key: "api-key"},
		},
	}
}

func singleFieldFields(v *vault.Vault, service, key, lang string) dialogDef {
	title := fmt.Sprintf("Скриня v%s — %s — %s", version, service, key)
	btnLabel := "Зберегти"
	if lang == "en" {
		title = fmt.Sprintf("Skrynia v%s — %s — %s", version, service, key)
		btnLabel = "Save"
	}
	existing := ""
	if val, err := v.Get(service, key); err == nil {
		existing = val
	}
	return dialogDef{
		title: title,
		btnLabel: btnLabel,
		fields: []field{
			{label: key, value: existing, key: key},
		},
	}
}

func runWin32Dialog(v *vault.Vault, service, lang string, def dialogDef) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	dlgState = dialogState{pwdEdits: make(map[syscall.Handle]bool)}

	hInstance, _, _ := getModuleHandle.Call(0)
	className := utf16Ptr("SkryniaDialog")

	cursor, _, _ := loadCursor.Call(0, uintptr(idrArrow))
	bgBrush, _, _ := getSysColorBrush.Call(colorBtnFace)

	wc := wndClassExW{
		size:       uint32(unsafe.Sizeof(wndClassExW{})),
		style:      3, // CS_HREDRAW | CS_VREDRAW
		wndProc:    syscall.NewCallback(wndProc),
		instance:   syscall.Handle(hInstance),
		cursor:     syscall.Handle(cursor),
		background: syscall.Handle(bgBrush),
		className:  className,
	}
	registerClassExW.Call(uintptr(unsafe.Pointer(&wc)))

	// Calculate window size based on fields
	winW := 420
	winH := 60 + len(def.fields)*60 + 50

	// Center on screen
	screenW, _, _ := getSystemMetrics.Call(smCxScreen)
	screenH, _, _ := getSystemMetrics.Call(smCyScreen)
	x := (int(screenW) - winW) / 2
	y := (int(screenH) - winH) / 2

	style := uintptr(wsOverlapped | wsCaption | wsSysMenu)

	hwnd, _, _ := createWindowExW.Call(
		0,
		uintptr(unsafe.Pointer(className)),
		uintptr(unsafe.Pointer(utf16Ptr(def.title))),
		style,
		uintptr(x), uintptr(y), uintptr(winW), uintptr(winH),
		0, 0, hInstance, 0,
	)

	// Set icon from embedded PNG
	if icon := loadEmbeddedIcon(); icon != 0 {
		sendMessage.Call(hwnd, wmSetIcon, iconSmall, icon)
		sendMessage.Call(hwnd, wmSetIcon, iconBig, icon)
	}

	// Get default font
	hFont := getDefaultFont()

	// Create controls
	yPos := 10
	for i, f := range def.fields {
		// Label
		hLabel, _, _ := createWindowExW.Call(0,
			uintptr(unsafe.Pointer(utf16Ptr("STATIC"))),
			uintptr(unsafe.Pointer(utf16Ptr(f.label))),
			uintptr(wsChild|wsVisible|ssLeft),
			15, uintptr(yPos), uintptr(winW-40), 18,
			hwnd, 0, hInstance, 0,
		)
		sendMessage.Call(hLabel, wmSetFont, hFont, 1)
		yPos += 20

		// Edit
		editStyle := uintptr(wsChild | wsVisible | wsTabStop | wsBorder | esAutoHScroll)
		editW := uintptr(winW - 40)
		if f.password {
			editStyle |= esPassword
			editW = uintptr(winW - 75) // leave room for eye button
		}
		hEdit, _, _ := createWindowExW.Call(0,
			uintptr(unsafe.Pointer(utf16Ptr("EDIT"))),
			uintptr(unsafe.Pointer(utf16Ptr(f.value))),
			editStyle,
			15, uintptr(yPos), editW, 24,
			hwnd, uintptr(idFirstEdit+i), hInstance, 0,
		)
		sendMessage.Call(hEdit, wmSetFont, hFont, 1)
		dlgState.hwndEdits = append(dlgState.hwndEdits, syscall.Handle(hEdit))

		if f.password {
			// Eye button to toggle password visibility
			hEye, _, _ := createWindowExW.Call(0,
				uintptr(unsafe.Pointer(utf16Ptr("BUTTON"))),
				uintptr(unsafe.Pointer(utf16Ptr("👁"))),
				uintptr(wsChild|wsVisible|wsTabStop|bsPushButton),
				editW+20, uintptr(yPos), 30, 24,
				hwnd, uintptr(idFirstEye+i), hInstance, 0,
			)
			sendMessage.Call(hEye, wmSetFont, hFont, 1)
			dlgState.pwdEdits[syscall.Handle(hEdit)] = true // currently masked
			_ = hEye
		}
		yPos += 36
	}

	// Save button
	hBtn, _, _ := createWindowExW.Call(0,
		uintptr(unsafe.Pointer(utf16Ptr("BUTTON"))),
		uintptr(unsafe.Pointer(utf16Ptr(def.btnLabel))),
		uintptr(wsChild|wsVisible|wsTabStop|bsDefPushButton),
		uintptr(winW-115), uintptr(yPos), 90, 30,
		hwnd, idSaveBtn, hInstance, 0,
	)
	sendMessage.Call(hBtn, wmSetFont, hFont, 1)

	// Focus first edit
	if len(dlgState.hwndEdits) > 0 {
		setFocus.Call(uintptr(dlgState.hwndEdits[0]))
	}

	showWindow.Call(hwnd, swShow)
	updateWindow.Call(hwnd)

	// Message loop with Tab support
	var m msg
	for {
		ret, _, _ := getMessage.Call(uintptr(unsafe.Pointer(&m)), 0, 0, 0)
		if ret == 0 {
			break
		}
		// IsDialogMessage handles Tab/Shift+Tab navigation between controls
		handled, _, _ := isDialogMessage.Call(hwnd, uintptr(unsafe.Pointer(&m)))
		if handled != 0 {
			continue
		}
		translateMessage.Call(uintptr(unsafe.Pointer(&m)))
		dispatchMessage.Call(uintptr(unsafe.Pointer(&m)))
	}

	// Save results
	if dlgState.saved {
		for i, f := range def.fields {
			if i < len(dlgState.results) && dlgState.results[i] != "" {
				if err := v.Set(service, f.key, dlgState.results[i]); err != nil {
					fmt.Fprintf(os.Stderr, "error: %v\n", err)
				}
			}
		}
	}
}

const (
	idOK     = 1 // Enter via IsDialogMessage
	idCancel = 2 // Esc via IsDialogMessage
)

func saveAndClose(hwnd syscall.Handle) {
	dlgState.saved = true
	dlgState.results = nil
	for _, hEdit := range dlgState.hwndEdits {
		dlgState.results = append(dlgState.results, getWindowText(hEdit))
	}
	destroyWindow.Call(uintptr(hwnd))
}

func wndProc(hwnd syscall.Handle, msg uint32, wParam, lParam uintptr) uintptr {
	switch msg {
	case wmCommand:
		id := int(wParam & 0xFFFF)
		switch {
		case id == idSaveBtn || id == idOK:
			saveAndClose(hwnd)
			return 0
		case id == idCancel:
			destroyWindow.Call(uintptr(hwnd))
			return 0
		case id >= idFirstEye && id < idFirstEye+100:
			// Toggle password visibility
			editIdx := id - idFirstEye
			if editIdx < len(dlgState.hwndEdits) {
				hEdit := dlgState.hwndEdits[editIdx]
				if dlgState.pwdEdits[hEdit] {
					// Show password
					sendMessage.Call(uintptr(hEdit), emSetPasswordChar, 0, 0)
					dlgState.pwdEdits[hEdit] = false
				} else {
					// Hide password
					sendMessage.Call(uintptr(hEdit), emSetPasswordChar, uintptr('●'), 0)
					dlgState.pwdEdits[hEdit] = true
				}
				invalidateRect.Call(uintptr(hEdit), 0, 1)
			}
			return 0
		}
	case wmClose:
		destroyWindow.Call(uintptr(hwnd))
		return 0
	case wmDestroy:
		postQuitMessage.Call(0)
		return 0
	}
	ret, _, _ := defWindowProc.Call(uintptr(hwnd), uintptr(msg), wParam, lParam)
	return ret
}

func getWindowText(hwnd syscall.Handle) string {
	length, _, _ := getWindowTextLength.Call(uintptr(hwnd))
	if length == 0 {
		return ""
	}
	buf := make([]uint16, length+1)
	getWindowTextW.Call(uintptr(hwnd), uintptr(unsafe.Pointer(&buf[0])), uintptr(length+1))
	return syscall.UTF16ToString(buf)
}

func utf16Ptr(s string) *uint16 {
	p, err := syscall.UTF16PtrFromString(s)
	if err != nil {
		// String contains NUL byte — use empty string as fallback.
		p, _ = syscall.UTF16PtrFromString("")
	}
	return p
}

func getDefaultFont() uintptr {
	gdiCreateFont := gdi32.NewProc("CreateFontW")
	hFont, _, _ := gdiCreateFont.Call(
		uintptr(uint32(0xFFFFFFF1)), // -15 (height)
		0, 0, 0,
		400, // FW_NORMAL
		0, 0, 0,
		1,   // DEFAULT_CHARSET
		0, 0, 5, // CLEARTYPE_QUALITY
		0,
		uintptr(unsafe.Pointer(utf16Ptr("Segoe UI"))),
	)
	return hFont
}

func loadEmbeddedIcon() uintptr {
	tmpFile, err := os.CreateTemp("", "skrynia-icon-*.ico")
	if err != nil {
		return 0
	}
	tmpPath := tmpFile.Name()
	tmpFile.Write(iconData)
	tmpFile.Close()
	defer os.Remove(tmpPath)

	loadImage := user32.NewProc("LoadImageW")
	icon, _, _ := loadImage.Call(
		0,
		uintptr(unsafe.Pointer(utf16Ptr(tmpPath))),
		1,    // IMAGE_ICON
		0, 0, // default size
		0x00000010, // LR_LOADFROMFILE
	)
	return icon
}
