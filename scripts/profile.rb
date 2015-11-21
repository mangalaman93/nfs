#!/usr/bin/ruby

## generates profile of a container for an experiment
#

require "flags"
require 'influxdb'

# constants
metrics = ["cpu_usage_total", "rx_bytes", "tx_bytes", "rxqueue_udp", "response_time", "current_calls"]
select_str = ["derivative(value)/10000000", "derivative(value)", "derivative(value)", "mean(value)", "mean(value)", "mean(value)"]
value_str = ["", "derivative", "derivative", "mean", "mean", "mean"]

# for snort
# metrics = ["cpu_usage_total", "memory_usage", "rx_bytes", "tx_bytes", "rx_packets", "tx_packets", "snort_queue_length", "snort_queue_drops", "snort_user_drops"]
# select_str = ["derivative(value)/10000000", "mean(value)", "derivative(value)", "derivative(value)", "derivative(value)", "derivative(value)", "mean(value)", "mean(value)", "mean(value)"]
# value_str = ["", "mean", "derivative", "derivative", "derivative", "derivative", "mean", "mean", "mean"]

Flags.define_string(:ip, "0.0.0.0", "server ip")
Flags.define_int(:port, 8086, "port")
Flags.define_string(:user, "root", "username")
Flags.define_string(:pass, "root", "password")
Flags.define_string(:db, "database", "database")
Flags.define_string(:cont, "cont", "name of container")
Flags.define_int(:from, 2345677, "from time")
Flags.define_int(:to, 2345888, "to time")
Flags.init

if Flags.flags_is_default(:db)
  puts Flags.help_message
  raise "incorrect db argument!"
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
cont   = Flags.cont
from   = Flags.from/1000
to     = Flags.to/1000

# influxdb client
influxdb = InfluxDB::Client.new db,
  host: ip,
  port: port,
  username: user,
  password: pass

# fetch call rate data
call_rates = []
timestamps = []
sum = 0
count = 0
query = "SELECT * FROM call_rate WHERE time > #{from}s AND time < #{to}s AND container_name = '#{cont}'"
influxdb.query query, epoch: 'ms' do |name, tags, points|
  points.each { |point|
    rate = point["value"].to_i

    # init
    if call_rates.empty?
      call_rates = [rate]
      timestamps = [point["time"]]
    end

    # next
    if (call_rates.last - rate).abs > 130
      sum = 0
      count = 0
      call_rates.push(rate)
      timestamps.push(point["time"])
    end

    # iter
    sum += rate
    count += 1
    call_rates[call_rates.length-1] = sum/count
  }
end

# print first row
print "\t"
metrics.each { |m|
  print "#{m}\t"
}

# calculate averages for each interval
timestamps.each_with_index { |t, i|
  if i == (timestamps.length-1)
    break
  end

  print "\n#{call_rates[i]}\t"
  metrics.each_with_index { |m, j|
    sum = 0
    count = 0
    query = "SELECT #{select_str[j]} FROM #{m} WHERE time > '#{t}' AND time < '#{timestamps[i+1]}' AND container_name = '#{cont}' GROUP BY time(1s)"
    data = influxdb.query query do |name, tags, points|
      points.each { |point|
        sum += point[value_str[j]].to_f
        count += 1
      }
    end

    print "#{sum/count}\t"
  }
}
print "\n"
