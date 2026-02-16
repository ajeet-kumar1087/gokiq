require 'json'

module Sidecar
  class Server
    def call(env)
      request = Rack::Request.new(env)

      case request.path
      when '/execute'
        handle_execute(request)
      when '/health'
        handle_health(request)
      else
        [404, { 'Content-Type' => 'application/json' }, [{ error: 'Not Found' }.to_json]]
      end
    rescue StandardError => e
      [500, { 'Content-Type' => 'application/json' }, [{ status: 'failure', error: e.message }.to_json]]
    end

    private

    def handle_execute(request)
      return [405, {}, ['Method Not Allowed']] unless request.post?

      payload = JSON.parse(request.body.read)
      job_class_name = payload['class']
      args = payload['args']
      jid = payload['jid']

      start_time = Time.now

      # In a real Rails app, we would do:
      # job_class = Object.const_get(job_class_name)
      # job_class.new(*args).perform
      
      # For now, let's simulate the execution
      $stdout.puts "[Sidecar] Executing Job: #{job_class_name} [#{jid}] with args: #{args.inspect}"
      $stdout.flush
      
      # Simulate work using the first argument if it's a number
      # In Sidekiq format, args is an array. Our enqueue script sends [name, duration]
      duration = args[1].is_a?(Numeric) ? args[1] : 0.1
      sleep duration 
      
      execution_time = Time.now - start_time

      [200, { 'Content-Type' => 'application/json' }, [{
        status: 'success',
        jid: jid,
        execution_time: execution_time
      }.to_json]]
    rescue JSON::ParserError
      [400, { 'Content-Type' => 'application/json' }, [{ error: 'Invalid JSON' }.to_json]]
    rescue NameError => e
      [404, { 'Content-Type' => 'application/json' }, [{ status: 'failure', error: "Job class not found: #{e.message}" }.to_json]]
    end

    def handle_health(request)
      [200, { 'Content-Type' => 'application/json' }, [{ status: 'ok', timestamp: Time.now.to_i }.to_json]]
    end
  end
end
