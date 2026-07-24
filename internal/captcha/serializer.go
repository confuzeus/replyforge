package captcha

import (
	"encoding/base64"
	"fmt"
	"math/big"
)

func BigIntToBase64(n *big.Int) string {
	if n == nil || n.Sign() == 0 {
		return ""
	}
	bytes := n.Bytes()
	return base64.RawStdEncoding.EncodeToString(bytes)
}

func Base64ToBigInt(s string) (*big.Int, error) {
	if s == "" {
		return nil, fmt.Errorf("captcha: empty base64 string")
	}
	bytes, err := base64.RawStdEncoding.DecodeString(s)
	if err != nil {
		return nil, fmt.Errorf("captcha: decoding base64: %w", err)
	}
	return new(big.Int).SetBytes(bytes), nil
}
