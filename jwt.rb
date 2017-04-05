require 'openssl'
require 'active_support'
require 'active_support/core_ext/numeric'
require 'jwt'
# Private key contents
private_pem = File.read("/home/jdyson/Downloads/pure-bot.2017-04-04 (1).private-key.pem")
private_key = OpenSSL::PKey::RSA.new(private_pem)

# Generate the JWT
payload = {
  # issued at time
  iat: Time.now.to_i,
  # JWT expiration time (10 minute maximum)
  exp: 10.minutes.from_now.to_i,
  # Integration's GitHub identifier
  iss: 1949
}

print JWT.encode(payload, private_key, "RS256")

