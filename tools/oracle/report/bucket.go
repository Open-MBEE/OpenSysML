package report

// Bucket is the verdict a suite referee files a test under.
type Bucket string

// The four buckets, in report order.
const (
	BucketPass            Bucket = "pass"
	BucketFail            Bucket = "fail"
	BucketNotExpressible  Bucket = "not-expressible"
	BucketDiffersByDesign Bucket = "differs-by-design"
)

// Buckets lists every bucket in report order.
var Buckets = []Bucket{BucketPass, BucketFail, BucketNotExpressible, BucketDiffersByDesign}
