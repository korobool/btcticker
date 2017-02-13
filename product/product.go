package product

type ProductType int

const (
	_ ProductType = iota
	ProductBtcUsd
	ProductBtcEur
	ProductEurUsd
)

func ProductToString(product ProductType) string {
	switch product {
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
