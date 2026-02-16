require 'grpc'
require_relative 'job_execution_services_pb'

module Sidecar
  class GRPCServer < JobExecution::JobExecution::Service
    def execute_job(job_req, _unused_call)
      start_time = Time.now
      
      # Log job execution
      $stdout.puts "[gRPC Sidecar] Executing Job: #{job_req.class} [#{job_req.jid}]"
      $stdout.flush
      
      # Simulate work based on args (simplification: assume first arg is duration)
      duration = job_req.args.first.to_f
      duration = 0.1 if duration <= 0
      sleep duration
      
      execution_time = Time.now - start_time
      
      JobExecution::JobResponse.new(
        status: "success",
        jid: job_req.jid,
        execution_time: execution_time
      )
    rescue StandardError => e
      JobExecution::JobResponse.new(
        status: "failure",
        jid: job_req.jid,
        error_message: e.message
      )
    end

    def health_check(health_req, _unused_call)
      JobExecution::HealthResponse.new(status: "ok", rails_loaded: true)
    end
  end
end
