package models

type URLType int64

const (
	Seed URLType = iota
	Asset
)

func (t URLType) String() string {
	switch t {
	case Seed:
		return "seed"
	case Asset:
		return "asset"
	}

	return ""
}
