package version

var (
	Version = "development"
	Commit  = "unknown"
	Date    = "unknown"
)

func Info() string {
	return Version
}
