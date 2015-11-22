#!/usr/bin/ruby

## computes rtt for output of profile.sh
#

require "csv"

unless ARGV.length == 1
  puts "usage: ruby rtts.rb <folder>"
  exit
end

folder = ARGV[0]
if folder[-1, 1] != "/"
  puts "end slash expected but not found"
  exit
end

# result["shares-rate-(uac/uas)"]
mean = {}
std = {}
rates = []
shares = []

Dir.glob("#{folder}**/*.csv") do |file|
  unless file.include? "rtt"
    next
  end

  puts "reading file #{file}"
  count = 0
  sum = 0
  sums = 0
  CSV.foreach(file, {:col_sep => ";"}) do |row|
  if count == 0
    count += 1
    next
  end
  num = row[1].to_f
  count += 1
  sum += num
  sums += (num*num)
  end

  s1 = file.split(folder)[1]
  s2 = s1.split("/volumes")[0]
  index = s2 + "-" + s1.split("/_data/")[1][0,3]
  s3 = s2.split("-")
  shares.push(s3[0].to_i)
  rates.push(s3[1].to_i)

  count -= 1
  mean[index] = sum/count
  std[index] = Math.sqrt(sums/count - (sum/count)*(sum/count))
end

shares.uniq!
shares.sort!
rates.uniq!
rates.sort!

[mean, std].zip(["mean", "std"]).each do |arr, name|
  for type in ["client", "server"]
    puts "#{type} response time (#{name})"

    print "\t"
    for rate in rates
      print rate, "\t"
    end
    print "\n"

    for share in shares
      print "#{share}\t"
      for rate in rates
        print arr["#{share}-#{rate}-ua#{type[0]}"], "\t"
      end
      print "\n"
    end
    print "\n"
  end
end
