package product

type ProductType int

const (
	_ ProductType = iota
	ProductBtcUsd
	ProductBtcEur
	ProductEurUsd
)

func (p ProductType) String() string {
	switch p {
	case ProductBtcUsd:
		return "BTC/USD"
	case ProductBtcEur:
		return "BTC/EUR"
	case ProductEurUsd:
		return "EUR/USD"
	default:
		return "UNKNOWN"
	}
}
