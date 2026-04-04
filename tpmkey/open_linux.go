//go:build !windows

package tpmkey

import (
	"github.com/google/go-tpm/tpm2/transport"
	"github.com/google/go-tpm/tpm2/transport/linuxtpm"
)

const tpmPath = "/dev/tpmrm0"

func openTPM() (transport.TPMCloser, error) {
	return linuxtpm.Open(tpmPath)
}
