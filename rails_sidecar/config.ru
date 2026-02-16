require 'json'
require_relative 'lib/sidecar/server'

# Load Rails environment from the main app
# In a real setup, we would point to the absolute path of the rails_app
# For this prototype, we'll assume the sidecar has access to the app code
# require_relative '../../rails_app/config/environment'

run Sidecar::Server.new
