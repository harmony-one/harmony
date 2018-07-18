for file in $(ls *leader*); do echo $file; cat $file | grep TPS | head -n 2 |  cut -f2 -d ":" | cut -f1 -d "," | awk '{ sum += $1; n++ } END { if (n > 0) print sum / n; }'; done
