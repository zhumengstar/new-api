package model

// InitColumnNamesForTest initializes package-level quoted column names for tests
// that construct an in-memory DB without running the full application init path.
func InitColumnNamesForTest() {
	initCol()
}
