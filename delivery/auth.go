package delivery

import (
	"crypto/sha512"
	"fmt"
	"strings"
)

var (
	DefaultTokenHashSalt = "ca205de6a03db4bf5376facdb172cb79"
	DefaultTokenHash     = tokenHash
)

func tokenHash(menuNames []string) string {
	return fmt.Sprintf("%x", sha512.Sum512([]byte(strings.Join(menuNames, ",")+DefaultTokenHashSalt)))
}
