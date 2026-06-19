package memory

// Change describes a memory write performed during extraction.
type Change struct {
	Action             string
	Kind               string
	PreferenceCategory string
	Content            string
	ID                 string
}
