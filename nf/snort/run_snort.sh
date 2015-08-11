export snort_path=/usr/local
export LUA_PATH=$snort_path/include/snort/lua/\?.lua\;\;
export SNORT_LUA_PATH=$snort_path/etc/snort

mkdir -p $snort_path/var/log/snort/
snort -c $snort_path/etc/snort/snort.lua -R $snort_path/etc/snort/sample.rules --max-packet-threads 8 -i eth0 -L dump -l $snort_path/var/log/snort/ -D
