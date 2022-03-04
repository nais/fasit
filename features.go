package fasit

import "embed"

//go:embed features/*.yaml
var FeaturesFS embed.FS
