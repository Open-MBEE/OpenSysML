package grpc

import (
	"context"
	"fmt"
	"strings"
	"testing"

	pb "github.com/Open-MBEE/OpenSysML/api/proto"
)

func BenchmarkImportCollisionScale(b *testing.B) {
	for _, n := range []int{256, 512, 1024, 2048} {
		b.Run(fmt.Sprint(n), func(b *testing.B) {
			var text strings.Builder
			text.WriteString("package Catalog {\n")
			for i := 0; i < n; i++ {
				fmt.Fprintf(&text, "package P%d { part sampleMember; }\n", i)
			}
			text.WriteString("}\npackage Consumer { private import Catalog::**; }\n")
			svc, err := NewService(2, "benchmark")
			if err != nil {
				b.Fatal(err)
			}
			defer svc.Close()
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				content := fmt.Sprintf("%s\n// %d", text.String(), i)
				_, err := svc.ParseSources(context.Background(), &pb.ParseSourcesRequest{Documents: []*pb.SourceDocument{{Name: "catalog.sysml", Source: &pb.SourceDocument_Content{Content: content}}}})
				if err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
