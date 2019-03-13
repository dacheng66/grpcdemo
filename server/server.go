package main


import (
	"sync"
	"github.com/golang/protobuf/proto"
	pb "grpcdemo/grpcdemo"
	"context"
	"fmt"
	"net"
	"flag"
	"google.golang.org/grpc"
	"log"
	"io/ioutil"
	"encoding/json"
	"math"
	"time"
	"io"
)

var (
	jsonDBFile = flag.String("json_db_file", "testdata/" +
		"route_guide_db.json", "A json file containing a list of features")
	port       = flag.Int("port", 10000, "The server port")
)


type grpcDemoServer struct {
	savedFeatures []*pb.Feature // read-only after initialized
	mu         sync.Mutex // protects routeNotes

	routeNotes map[string][]*pb.RouteNote
}

//简单 RPC
func (s *grpcDemoServer) GetFeature(ctx context.Context, point *pb.Point) (*pb.Feature, error) {
	fmt.Println("简单RPC")
	defer fmt.Println("简单RPC结束了")
	for _, feature := range s.savedFeatures {
		if proto.Equal(feature.Location, point) {
			return feature, nil
		}
	}
	// No feature was found, return an unnamed feature
	return &pb.Feature{Location: point}, nil
}

//服务器端流式 RPC
//请求对象是一个 Rectangle，客户端期望从中找到 Feature
// 这次我们得到了一个请求对象和一个特殊的RouteGuide_ListFeaturesServer来写入我们的响应，而不是得到方法参数中的简单请求和响应对象。
//在这个方法中，我们填充了尽可能多的 Feature 对象去返回，用它们的 Send() 方法把它们写入 RouteGuide_ListFeaturesServer。
// 最后，在我们的简单 RPC中，我们返回了一个 nil 错误告诉 gRPC 响应的写入已经完成。
func (s *grpcDemoServer) ListFeatures(rect *pb.Rectangle, stream pb.GrpcDemo_ListFeaturesServer) error {
	for _, feature := range s.savedFeatures {
		if inRange(feature.Location, rect) {
			if err := stream.Send(feature); err != nil {
				return err
			}
		}
	}
	return nil
}

//客户端流式 RPC
//客户端流方法 RecordRoute，我们通过它可以从客户端拿到一个 Point 的流，其中包括它们路径的信息
//这个方法没有请求参数。相反的，它拿到了一个 RouteGuide_RecordRouteServer 流，
// 服务器可以用它来同时读 和 写消息——它可以用自己的 Recv() 方法接收客户端消息并且用 SendAndClose() 方法返回它的单个响应。
func (s *grpcDemoServer) RecordRoute(stream pb.GrpcDemo_RecordRouteServer) error {
	var pointCount, featureCount, distance int32
	var lastPoint *pb.Point
	startTime := time.Now()
	for {
		point, err := stream.Recv()
		if err == io.EOF {
			endTime := time.Now()
			return stream.SendAndClose(&pb.RouteSummary{
				PointCount:   pointCount,
				FeatureCount: featureCount,
				Distance:     distance,
				ElapsedTime:  int32(endTime.Sub(startTime).Seconds()),
			})
		}
		if err != nil {
			return err
		}
		pointCount++
		for _, feature := range s.savedFeatures {
			if proto.Equal(feature.Location, point) {
				featureCount++
			}
		}
		if lastPoint != nil {
			distance += calcDistance(lastPoint, point)
		}
		lastPoint = point
	}
}

//双向流式 RPC
//服务器会使用流的 Send() 方法而不是 SendAndClose()，因为它需要写多个响应。
// 虽然客户端和服务器端总是会拿到对方写入时顺序的消息，它们可以以任意顺序读写——流的操作是完全独立的。
func (s *grpcDemoServer) RouteChat(stream pb.GrpcDemo_RouteChatServer) error {
	for {
		in, err := stream.Recv()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		key := serialize(in.Location)

		s.mu.Lock()
		s.routeNotes[key] = append(s.routeNotes[key], in)
		// Note: this copy prevents blocking other clients while serving this one.
		// We don't need to do a deep copy, because elements in the slice are
		// insert-only and never modified.
		rn := make([]*pb.RouteNote, len(s.routeNotes[key]))
		copy(rn, s.routeNotes[key])
		s.mu.Unlock()

		for _, note := range rn {
			if err := stream.Send(note); err != nil {
				return err
			}
		}
	}
}

func main() {
	lis, err := net.Listen("tcp", fmt.Sprintf("localhost:%d", *port))
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}
	grpcServer := grpc.NewServer()
	pb.RegisterGrpcDemoServer(grpcServer,newServer())
	grpcServer.Serve(lis)
}

//以下均为辅助函数
//简单 RPC 辅助函数
func (s *grpcDemoServer) loadFeatures(filePath string) {
	var data []byte
	var err error
	data, err = ioutil.ReadFile(filePath)
	if err != nil {
		log.Fatalf("Failed to load default features: %v", err)
	}

	if err := json.Unmarshal(data, &s.savedFeatures); err != nil {
		log.Fatalf("Failed to load default features: %v", err)
	}
}

func newServer() *grpcDemoServer{
	s := &grpcDemoServer{routeNotes: make(map[string][]*pb.RouteNote)}
	s.loadFeatures(*jsonDBFile)
	return s
}

//服务器端流式 RPC 辅助函数
func inRange(point *pb.Point, rect *pb.Rectangle) bool {
	left := math.Min(float64(rect.Lo.Longitude), float64(rect.Hi.Longitude))
	right := math.Max(float64(rect.Lo.Longitude), float64(rect.Hi.Longitude))
	top := math.Max(float64(rect.Lo.Latitude), float64(rect.Hi.Latitude))
	bottom := math.Min(float64(rect.Lo.Latitude), float64(rect.Hi.Latitude))

	if float64(point.Longitude) >= left &&
		float64(point.Longitude) <= right &&
		float64(point.Latitude) >= bottom &&
		float64(point.Latitude) <= top {
		return true
	}
	return false
}

//客户端流式 RPC 辅助函数
func toRadians(num float64) float64 {
	return num * math.Pi / float64(180)
}

func calcDistance(p1 *pb.Point, p2 *pb.Point) int32 {
	const CordFactor float64 = 1e7
	const R = float64(6371000) // earth radius in metres
	lat1 := toRadians(float64(p1.Latitude) / CordFactor)
	lat2 := toRadians(float64(p2.Latitude) / CordFactor)
	lng1 := toRadians(float64(p1.Longitude) / CordFactor)
	lng2 := toRadians(float64(p2.Longitude) / CordFactor)
	dlat := lat2 - lat1
	dlng := lng2 - lng1

	a := math.Sin(dlat/2)*math.Sin(dlat/2) +
		math.Cos(lat1)*math.Cos(lat2)*
			math.Sin(dlng/2)*math.Sin(dlng/2)
	c := 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))

	distance := R * c
	return int32(distance)
}



//双向流式 RPC 辅助函数
func serialize(point *pb.Point) string {
	return fmt.Sprintf("%d %d", point.Latitude, point.Longitude)
}