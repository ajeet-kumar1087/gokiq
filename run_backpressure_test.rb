require 'redis'
require 'json'
require 'securerandom'

redis = Redis.new(url: ENV['REDIS_URL'] || 'redis://localhost:6379/0')
JOB_COUNT = 10

def clear_queues(redis)
  redis.del('queue:default')
end

def enqueue_batch(redis, count)
  puts "Enqueuing #{count} jobs with 2.0s work duration..."
  count.times do |i|
    job = {
      'class' => 'HardWorkJob',
      'args' => ['User', 2.0],
      'jid' => "job-#{i+1}",
      'queue' => 'default',
      'created_at' => Time.now.to_f,
      'enqueued_at' => Time.now.to_f
    }.to_json
    redis.lpush('queue:default', job)
  end
end

clear_queues(redis)
enqueue_batch(redis, JOB_COUNT)
puts "Jobs enqueued. Check Go Worker logs to see only 3 running at a time."
