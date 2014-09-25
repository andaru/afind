package afind

// Afind configuration file definition and handling

type Config struct {
	Noindex     string // regular expression of files not to index
	IndexRoot   string // path to root of index files
	IndexInRepo bool   // prefix IndexRoot with the repo root directory?
	BindFlag    string
	RpcBindFlag string
}

var (
	config Config
)

func SetConfig(c Config) {
	config = c
}
