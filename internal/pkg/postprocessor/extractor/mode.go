package extractor

type Mode int

const (
	ModeGeneral  Mode = iota // Zeno is running in general archiving mode
	ModeHeadless             // Zeno is running in headless browser mode
)
