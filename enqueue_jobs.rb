require 'redis'
require 'json'
require 'securerandom'

redis = Redis.new(url: ENV['REDIS_URL'] || 'redis://localhost:6379/0')

def enqueue_job(redis, klass, args)
  job = {
    'class' => klass,
    'args' => args,
    'jid' => SecureRandom.hex(12),
    'queue' => 'default',
    'created_at' => Time.now.to_f,
    'enqueued_at' => Time.now.to_f
  }
  
  redis.lpush('queue:default', job.to_json)
  puts "Enqueued #{klass} with JID #{job['jid']}"
end

# Enqueue a few jobs
enqueue_job(redis, 'HardWorkJob', ['Alice', 2])
enqueue_job(redis, 'HardWorkJob', ['Bob', 1])
enqueue_job(redis, 'HardWorkJob', ['Charlie', 3])
