#!/usr/bin/ruby

require 'csv'

unless ARGV.length == 1
  puts "usage: ruby rtt_stats.rb <file>"
  exit
end

file = ARGV[0]
puts "reading file #{file}"
other_count = 0
count = 0
sum = 0
sums = 0
CSV.foreach(file, {:col_sep => ";"}) do |row|
  if count == 0
    count += 1
    next
  end
  num = row[1].to_f
  if num > 1000
    other_count += 1
    next
  end
  count += 1
  sum += num
  sums += (num*num)
end

count -= 1
puts "sum:#{sum} sums:#{sums} count:#{count} other_count:#{other_count}"
puts "mean:#{sum/count}, std:#{Math.sqrt(sums/count - (sum/count)*(sum/count))}"
