package main

import (
	"context"
	"crypto/sha256"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/atotto/clipboard"
	"github.com/justwatchcom/gopass/store/sub"
	"github.com/marcusolsson/tui-go"
)

const logo = `
  ________                                   
 /  _____/  ____ ___________    ______ ______
/   \  ___ /  _ \\____ \__  \  /  ___//  ___/
\    \_\  (  <_> )  |_> > __ \_\___ \ \___ \ 
 \______  /\____/|   __(____  /____  >____  >
        \/       |__|       \/     \/     \/ `

func main() {
	unclipFlag := flag.Bool("u", false, "")
	clipboardTimeout := flag.Int("t", 15, "Clipboard timeout in seconds")
	flag.Parse()

	// Process spawned to clear clipboard
	if *unclipFlag {
		unclip(*clipboardTimeout)
		os.Exit(0)
	}

	store := sub.New("root", os.Getenv("HOME")+"/.password-store")
	secrets, err := store.List("")
	if err != nil {
		panic(err)
	}

	status := tui.NewStatusBar("Enter - Copy to clipboard, Right Arrow - Show, ESC - Quit.")

	storeList := tui.NewList()
	storeList.AddItems(secrets...)
	storeList.SetFocused(true)
	storeList.SetSelected(0)

	search := tui.NewEntry()
	search.SetFocused(true)
	search.OnChanged(func(entry *tui.Entry) {
		foundItems := make([]string, 0)
		allItems, err := store.List("")
		if err != nil {
			status.SetText(err.Error())
		}
		for _, item := range allItems {
			if strings.Contains(item, entry.Text()) {
				foundItems = append(foundItems, item)
			}
		}
		storeList.RemoveItems()
		storeList.AddItems(foundItems...)
		if len(foundItems) > 0 {
			storeList.SetSelected(0)
		}
	})

	form := tui.NewGrid(0, 0)
	form.AppendRow(tui.NewLabel("Search:"))
	form.AppendRow(search)

	window := tui.NewVBox(
		tui.NewPadder(10, 1, tui.NewLabel(logo)),
		tui.NewPadder(1, 1, storeList),
		tui.NewPadder(1, 1, form),
	)
	window.SetBorder(true)

	wrapper := tui.NewVBox(
		tui.NewSpacer(),
		window,
		tui.NewSpacer(),
	)

	content := tui.NewHBox(tui.NewSpacer(), wrapper, tui.NewSpacer())

	root := tui.NewVBox(
		content,
		status,
	)

	ui := tui.New(root)
	ui.SetKeybinding("Esc", func() { ui.Quit() })

	// Show password
	ui.SetKeybinding("Right", func() {
		secret, err := store.Get(context.Background(), storeList.SelectedItem())
		if err != nil {
			status.SetText(err.Error())
			return
		}
		ui.Quit()
		fmt.Printf("Password for %v: %v\n", storeList.SelectedItem(), secret.String())
	})

	// Copy to clipboard
	storeList.OnItemActivated(func(list *tui.List) {
		secret, err := store.Get(context.Background(), list.SelectedItem())
		if err != nil {
			status.SetText(err.Error())
			return
		}

		if err := clipboard.WriteAll(secret.String()); err != nil {
			panic(err)
		}

		err = delayedClearClipboard(secret.String(), *clipboardTimeout)
		if err != nil {
			panic(err)
		}

		ui.Quit()
		fmt.Printf("Password for %v copied to clipboard. Will clear in %v seconds.\n",
			storeList.SelectedItem(),
			*clipboardTimeout)
	})

	if err := ui.Run(); err != nil {
		panic(err)
	}
}

func delayedClearClipboard(content string, timeout int) error {
	hash := fmt.Sprintf("%x", sha256.Sum256([]byte(content)))

	cmd := exec.Command(os.Args[0], "-u", strconv.Itoa(timeout))
	// https://groups.google.com/d/msg/golang-nuts/shST-SDqIp4/za4oxEiVtI0J
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setpgid: true,
	}
	cmd.Env = append(os.Environ(), "GOPASS_TUI_UNCLIP_CHECKSUM="+hash)
	return cmd.Start()
}

func unclip(timeout int) {
	checksum := os.Getenv("GOPASS_TUI_UNCLIP_CHECKSUM")
	time.Sleep(time.Second * time.Duration(timeout))

	curr, err := clipboard.ReadAll()
	if err != nil {
		panic(err)
	}

	hash := fmt.Sprintf("%x", sha256.Sum256([]byte(curr)))

	if hash == checksum {
		if err := clipboard.WriteAll(""); err != nil {
			panic(err)
		}
	}
}
