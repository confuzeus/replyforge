package captcha

import "math/big"

var ChallengeTypes = []string{"QuadraticResidueProblem"}

var Woodalls = map[string]*big.Int{
	"751*2^751-1":     new(big.Int).Sub(new(big.Int).Mul(big.NewInt(751), new(big.Int).Exp(big.NewInt(2), big.NewInt(751), nil)), big.NewInt(1)),
	"83*2^5318-1":     new(big.Int).Sub(new(big.Int).Mul(big.NewInt(83), new(big.Int).Exp(big.NewInt(2), big.NewInt(5318), nil)), big.NewInt(1)),
	"7755*2^7755-1":   new(big.Int).Sub(new(big.Int).Mul(big.NewInt(7755), new(big.Int).Exp(big.NewInt(2), big.NewInt(7755), nil)), big.NewInt(1)),
	"9531*2^9531-1":   new(big.Int).Sub(new(big.Int).Mul(big.NewInt(9531), new(big.Int).Exp(big.NewInt(2), big.NewInt(9531), nil)), big.NewInt(1)),
	"12379*2^12379-1": new(big.Int).Sub(new(big.Int).Mul(big.NewInt(12379), new(big.Int).Exp(big.NewInt(2), big.NewInt(12379), nil)), big.NewInt(1)),
	"7911*2^15823-1":  new(big.Int).Sub(new(big.Int).Mul(big.NewInt(7911), new(big.Int).Exp(big.NewInt(2), big.NewInt(15823), nil)), big.NewInt(1)),
	"18885*2^18885-1": new(big.Int).Sub(new(big.Int).Mul(big.NewInt(18885), new(big.Int).Exp(big.NewInt(2), big.NewInt(18885), nil)), big.NewInt(1)),
	"22971*2^22971-1": new(big.Int).Sub(new(big.Int).Mul(big.NewInt(22971), new(big.Int).Exp(big.NewInt(2), big.NewInt(22971), nil)), big.NewInt(1)),
}

var WoodallAliases = map[string]string{
	"2xs": "751*2^751-1",
	"xs":  "83*2^5318-1",
	"sm":  "7755*2^7755-1",
	"md":  "9531*2^9531-1",
	"lg":  "12379*2^12379-1",
	"xl":  "7911*2^15823-1",
	"2xl": "18885*2^18885-1",
	"3xl": "22971*2^22971-1",
}
