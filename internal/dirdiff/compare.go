package dirdiff

// compareContents settles the Modified entries whose two files have the
// same size: equal bytes make them Unchanged.
func compareContents(entries []Entry) {
	for i := range entries {
		e := &entries[i]
		if e.Status == Modified && e.Note == "" && e.LeftSize == e.RightSize {
			e.Status = Unchanged
		}
	}
}
