require 'redis'
require 'json'
require 'securerandom'

redis = Redis.new(url: ENV['REDIS_URL'] || 'redis://localhost:6379/0')
JOB_COUNT = 100_000
# Generate 1KB of dummy metadata (reduced from 5KB to avoid memory issues during enqueuing)
METADATA = "A" * 1024 

def clear_queues(redis)
  puts "Clearing Redis..."
  redis.del('queue:default')
  redis.del('stat:processed') # Clear stats if possible
end

def enqueue_batch(redis, count)
  puts "Preparing #{count} jobs..."
  count.times do |i|
    job = {
      'class' => 'HardWorkJob',
      'args' => ['User', 0.1, METADATA], # 0.1s sleep to simulate production
      'jid' => SecureRandom.hex(12),
      'queue' => 'default',
      'created_at' => Time.now.to_f,
      'enqueued_at' => Time.now.to_f
    }.to_json
    
    redis.lpush('queue:default', job)
    
    if (i + 1) % 10000 == 0
      puts "Enqueued #{i + 1} jobs..."
    end
  end
end

def wait_for_completion(redis, label)
  total = JOB_COUNT
  start_time = Time.now
  puts "
[#{label}] Benchmark started at #{start_time}"
  
  loop do
    size = redis.llen('queue:default')
    processed = total - size
    percentage = (processed.to_f / total * 100).round(1)
    print "Progress: #{processed}/#{total} (#{percentage}%) - Remaining: #{size}   "
    $stdout.flush
    break if size == 0
    sleep 1
  end
  
  end_time = Time.now
  duration = end_time - start_time
  puts "
[#{label}] Finished at #{end_time}"
  puts "[#{label}] Total Duration: #{duration.round(2)} seconds"
  puts "[#{label}] Throughput: #{(JOB_COUNT / duration).round(2)} jobs/sec"
  duration
end

# 1. Test Gokiq
puts "--- Testing Gokiq (Go Worker + Rails Sidecar) ---"
clear_queues(redis)
enqueue_batch(redis, JOB_COUNT)
gokiq_duration = wait_for_completion(redis, "Gokiq")

# In a real environment, we'd switch workers here. 
# For this script, the user should manage the docker containers.
# We'll just print the instructions.
puts "
Done with Gokiq. Please stop gokiq and start sidekiq_standard to compare."
