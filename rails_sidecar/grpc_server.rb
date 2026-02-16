require_relative 'lib/sidecar/grpc_server'
require_relative 'lib/job_execution_services_pb'

def main
  port = ENV.fetch('GRPC_PORT', '50051')
  url = "0.0.0.0:#{port}"
  
  s = GRPC::RpcServer.new
  s.add_http2_port(url, :this_port_is_insecure)
  s.handle(Sidecar::GRPCServer)
  
  $stdout.puts "[gRPC Sidecar] Server started on #{url}"
  $stdout.flush
  
  s.run_till_terminated_or_interrupted([1, 'int', 'sigint', 'term', 'sigterm'])
end

main if __FILE__ == $0
