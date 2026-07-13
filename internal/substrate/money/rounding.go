package money

import "math/big"

// RoundHalfUp divides numerator by denominator, rounding an exact half away
// from zero. It is used when an operation produces fractional minor units.
func RoundHalfUp(numerator, denominator int64) (int64, error) {
	rounded, err := roundHalfUp(big.NewInt(numerator), big.NewInt(denominator))
	if err != nil {
		return 0, err
	}
	if !rounded.IsInt64() {
		return 0, ErrOverflow
	}
	return rounded.Int64(), nil
}

func roundHalfUp(numerator, denominator *big.Int) (*big.Int, error) {
	if denominator.Sign() == 0 {
		return nil, ErrDivisionByZero
	}
	negative := numerator.Sign() != denominator.Sign()
	absNumerator := new(big.Int).Abs(numerator)
	absDenominator := new(big.Int).Abs(denominator)
	quotient, remainder := new(big.Int), new(big.Int)
	quotient.QuoRem(absNumerator, absDenominator, remainder)
	if new(big.Int).Mul(remainder, big.NewInt(2)).Cmp(absDenominator) >= 0 {
		quotient.Add(quotient, big.NewInt(1))
	}
	if negative && quotient.Sign() != 0 {
		quotient.Neg(quotient)
	}
	return quotient, nil
}
