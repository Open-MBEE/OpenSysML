package repl

import (
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"

	"google.golang.org/protobuf/encoding/protojson"

	pb "github.com/Open-MBEE/OpenSysML/api/proto"
	"github.com/Open-MBEE/OpenSysML/internal/exec/runtime"
	"github.com/Open-MBEE/OpenSysML/internal/protoconv"
)

// featuresUsage is how %features is written: an object, how far to expand what it
// holds, and whether to write the listing as text or as JSON.
const featuresUsage = "usage: %features <object> [all|depth <n>] [json]"

// maxFeatureGraphInstances bounds a JSON listing as maxFeatureValueLines bounds a
// text one, at the count the API serializes for one object.
var maxFeatureGraphInstances = protoconv.DefaultGraphBounds().Instances

// featureListing is how far %features expands an object, and in which form. A
// bound the user lifted is math.MaxInt.
type featureListing struct {
	depth  int // nested objects are expanded to this depth
	budget int // lines a text listing, or objects a JSON one, may spend
	json   bool
}

// parseFeatureListing reads the options after the object: `all` lifts every
// bound, `depth <n>` expands to that depth without a size bound, and `json`
// writes the graph the API's Instantiate returns. Without either, the listing
// is bounded as it always was.
func parseFeatureListing(opts []string) (featureListing, error) {
	listing := featureListing{depth: maxFeatureValueDepth, budget: maxFeatureValueLines}
	bounded := true
	for i := 0; i < len(opts); i++ {
		switch opts[i] {
		case "json":
			if listing.json {
				return listing, errors.New("json is given twice")
			}
			listing.json = true
		case "all":
			if !bounded {
				return listing, errors.New("all and depth <n> cannot both be given")
			}
			bounded = false
			listing.depth, listing.budget = math.MaxInt, math.MaxInt
		case "depth":
			if !bounded {
				return listing, errors.New("all and depth <n> cannot both be given")
			}
			if i+1 >= len(opts) {
				return listing, errors.New("depth needs a number")
			}
			n, err := strconv.Atoi(opts[i+1])
			if err != nil || n < 0 {
				return listing, fmt.Errorf("depth %q is not a non-negative integer", opts[i+1])
			}
			bounded = false
			listing.depth, listing.budget = n, math.MaxInt
			i++
		default:
			return listing, fmt.Errorf("unknown option %q", opts[i])
		}
	}
	if bounded && listing.json {
		listing.budget = maxFeatureGraphInstances
	}
	return listing, nil
}

// truncationHint tells a reader of a cut listing how to see the rest of it.
func (l featureListing) truncationHint(name string) string {
	return fmt.Sprintf("%%features %s all shows it whole, %%features %s depth <n> to a depth", name, name)
}

// featuresJSON writes the object and everything reachable from it as the API's
// Instantiate does, so a client reads one shape whether it asked the service or
// the REPL. A graph the object bound cut short says so as a warning, since the
// listing that comes back is a real answer about part of the run.
func (s *Session) featuresJSON(ctx *runtime.Context, inst *runtime.Instance, name string, listing featureListing) ([]string, bool, error) {
	bounds := protoconv.GraphBounds{Depth: listing.depth, Instances: listing.budget}
	graph := protoconv.InstanceGraphToProtoWithin(ctx, inst, s.symbolIndex(), bounds)

	resp := &pb.InstantiateResponse{Instance: graph.Root, Instances: graph.All}
	// A feature value the graph reports as an error is one the session could not
	// answer about, which a non-interactive run exits on.
	for _, err := range graph.Errors {
		s.noteMaterializationFailure(err)
	}
	if graph.Truncated {
		resp.Diagnostics = append(resp.Diagnostics, &pb.Diagnostic{
			Severity: "warning",
			Message: fmt.Sprintf("graph truncated at %d objects; %s",
				listing.budget, listing.truncationHint(name)),
		})
	}
	out, err := protojson.MarshalOptions{Multiline: true, Indent: "  "}.Marshal(resp)
	if err != nil {
		return nil, false, fmt.Errorf("features json: %w", err)
	}
	return strings.Split(strings.TrimRight(string(out), "\n"), "\n"), false, nil
}
