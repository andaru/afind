package afind

var IndexPathExcludes = newPathMatcher()

func init() {
	IndexPathExcludes.AddExtension(indexPathSuffix)
	IndexPathExcludes.AddExtension(".git")
	IndexPathExcludes.AddExtension(".hg")
	IndexPathExcludes.AddExtension(".svn")
}
