class HardWorkJob
  def perform(name, duration)
    puts "Starting hard work for #{name}..."
    sleep duration
    puts "Finished hard work for #{name}!"
    { status: 'completed', worker: 'rails_sidecar' }
  end
end
