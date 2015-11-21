#!/usr/bin/env ruby

## This scripts downloads data from influxdb with
## provided specifications (obvious)
#

require "flags"
require "json"
require 'tempfile'

Flags.define_string(:ip, "0.0.0.0", "server ip")
Flags.define_int(:port, 8086, "udp port")
Flags.define_string(:user, "root", "username")
Flags.define_string(:pass, "root", "password")
Flags.define_string(:db, "database", "database")
Flags.define_string(:series, "series", "series")
Flags.define_string(:cont, "cont", "name of container")
Flags.define_int(:from, 2345677, "from time")
Flags.define_int(:to, 2345888, "to time")
Flags.init

if Flags.flags_is_default(:db)
    puts Flags.help_message
    raise "incorrect db argument!"
end

if Flags.flags_is_default(:series)
    puts Flags.help_message
    raise "incorrect series argument!"
end

if Flags.flags_is_default(:cont)
    puts Flags.help_message
    raise "incorrect container_name argument!"
end


if Flags.flags_is_default(:from)
    puts Flags.help_message
    raise "incorrect from-time argument!"
end

if Flags.flags_is_default(:to)
    puts Flags.help_message
    raise "incorrect to-time argument!"
end

ip     = Flags.ip
port   = Flags.port
user   = Flags.user
pass   = Flags.pass
db     = Flags.db
series = Flags.series
cont   = Flags.cont
from   = Flags.from/1000
to     = Flags.to/1000

# run curl
cmd = "curl -G 'http://#{ip}:#{port}/query' -u #{user}:#{pass} --data-urlencode 'db=#{db}' --data-urlencode \"q=SELECT * FROM #{series} WHERE time > #{from}s AND time < #{to}s AND container_name = '#{cont}'\""
file = Tempfile.new('temp')
cmd = "#{cmd} > #{file.path}"
system(cmd)
raise "unable to run curl" unless $?.exitstatus == 0
data = JSON.parse(file.read)["results"][0]["series"][0]
file.close
file.unlink

# get the index for values
print "indexes present are: "
index = -1
data["columns"].each_with_index { |col, i|
    print "#{col}, "
    if col == "value"
        index = i
    end
}
print "\n"
raise "value not present!" if index == -1

File.open("#{series}.txt", 'w') do |f|
    data["values"].each { |item|
        f.puts item[index]
    }
end
puts "data written to #{series}.txt!"
