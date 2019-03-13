##### grpc快速入门
###### 1. .proto 文件中定义GrpcDemo服务

###### 2.在服务中定义 rpc 方法，指定请求的和响应类型。gRPC 允许你定义4种类型的 service 方法：
```go
      //简单 RPC ， 客户端使用存根发送请求到服务器并等待响应返回，就像平常的函数调用一样。
      rpc GetFeature(Point) returns (Feature) {}

      //服务器端流式 RPC ， 客户端发送请求到服务器，拿到一个流去读取返回的消息序列。
      //客户端读取返回的流，直到里面没有任何消息。
      rpc ListFeatures(Rectangle) returns (stream Feature) {}

      //客户端流式 RPC ， 客户端写入一个消息序列并将其发送到服务器，同样也是使用流。
      //一旦客户端完成写入消息，它等待服务器完成读取返回它的响应。
      rpc RecordRoute(stream Point) returns (RouteSummary) {}

      //双向流式 RPC 是双方使用读写流去发送一个消息序列。
      //两个流独立操作，因此客户端和服务器可以以任意喜欢的顺序读写
      rpc RouteChat(stream RouteNote) returns (stream RouteNote) {}
```

##### 3.生成外部调用类
```go
   //配置protoc-gen-go.exe，protoc.exe
   //cd E:\git_local\src\grpcdemo
   //protoc -I grpcdemo/ grpcdemo.proto --go_out=plugins=grpc:grpcdemo
```

##### 4.创建服务器server.go
      #注意：服务端的函数形参 stream pb.GrpcDemo_ListFeaturesServer

      
##### 5.创建客户端client.go
      #注意：客户端的函数形参 client pb.GrpcDemoClient