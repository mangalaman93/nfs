#!/usr/bin/env ruby

## Parses the output of `dstat -nf` and creates a spreadsheet
#

require 'spreadsheet'

unless ARGV.length == 2
  puts "usage: ruby netparse.rb data-file ws-file"
  exit
end

def get_interface(col)
    col = col.chomp('-')
    index = 0
    col.each_char { |c|
        if c == 'n'
            return col[index..-1]
        end
        index += 1
    }
end

book = Spreadsheet::Workbook.new
ws = book.create_worksheet

row_count = -1

file = File.open(ARGV[0], "r")
file.each_line do |row|
    row_count += 1
    col_count = 0
    if row_count == 0
        row.split(%r{[\s:]+}).each { |col|
            iface = get_interface(col)
            ws[row_count, col_count] = iface + "-send"
            col_count += 1
            ws[row_count, col_count] = iface + "-rec"
            col_count += 1
        }
    elsif row_count == 1
        next
    else
        row.split(%r{[\s:]+}).each { |col|
            unless col.nil?
                if col.strip != ""
                    ws[row_count-1, col_count] = col
                    col_count += 1
                end
            end
        }
    end
end

file.close
book.write "#{ARGV[1]}.xls"
