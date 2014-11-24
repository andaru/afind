package afind

var IndexPathExcludes = newPathMatcher()

const (
	indexPathSuffix = ".afindex"
)

func init() {
	IndexPathExcludes.AddExtension(indexPathSuffix)
	IndexPathExcludes.AddExtension(".git")
	IndexPathExcludes.AddExtension(".hg")
	IndexPathExcludes.AddExtension(".svn")
}
