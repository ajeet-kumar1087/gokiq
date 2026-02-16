require 'redis'
require 'json'
require 'securerandom'

redis = Redis.new(url: ENV['REDIS_URL'] || 'redis://localhost:6379/0')
JOB_COUNT = 20000
# Generate 5KB of dummy metadata to increase serialization/transfer overhead
METADATA = "A" * 5120 

def clear_queues(redis)
  puts "Clearing Redis..."
  redis.del('queue:default')
end

def enqueue_batch(redis, count)
  puts "Preparing #{count} jobs with 5KB payloads..."
  jobs = []
  count.times do |i|
    jobs << {
      'class' => 'HardWorkJob',
      'args' => ['User', 0.1, METADATA],
      'jid' => SecureRandom.hex(12),
      'queue' => 'default',
      'created_at' => Time.now.to_f,
      'enqueued_at' => Time.now.to_f
    }.to_json
    
    if (i + 1) % 5000 == 0
      puts "Prepared #{i + 1} jobs..."
    end
  end
  
  puts "Pushing batches to Redis..."
  jobs.each_slice(1000).with_index do |slice, index|
    redis.lpush('queue:default', slice)
    puts "Pushed batch #{index + 1}..."
  end
end

def wait_for_completion(redis)
  total = redis.llen('queue:default')
  start_time = Time.now
  puts "Benchmark started at #{start_time}"
  
  loop do
    size = redis.llen('queue:default')
    processed = total - size
    percentage = (processed.to_f / total * 100).round(1)
    print "\rProgress: #{processed}/#{total} (#{percentage}%) - Remaining: #{size}   "
    $stdout.flush
    break if size == 0
    sleep 0.5
  end
  
  end_time = Time.now
  duration = end_time - start_time
  puts "\nFinished at #{end_time}"
  puts "Total Duration: #{duration.round(2)} seconds"
  puts "Throughput: #{(JOB_COUNT / duration).round(2)} jobs/sec"
end

clear_queues(redis)
enqueue_batch(redis, JOB_COUNT)
wait_for_completion(redis)
