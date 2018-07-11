if [ $# -eq 0 ]; then
    echo "Please the directory of the log"
    exit 1
fi
DIR=$1
for file in $(ls $DIR/*leader*)
do
  cat $file | egrep -o "TPS=[0-9]+" | cut -f2 -d "=" | awk '{ sum += $1; n++ } END { if (n > 0) print sum / n; }';
done