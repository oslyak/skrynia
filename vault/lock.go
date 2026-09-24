package vault

import (
	"fmt"
	"os"
)

func lockVault(basePath string) (*os.File, error) {
	f, err := os.OpenFile(basePath+".lock", os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return nil, err
	}
	if err := lockFile(f); err != nil {
		f.Close()
		return nil, fmt.Errorf("vault: lock: %w", err)
	}
	return f, nil
}
