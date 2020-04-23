#!/bin/bash
function echodir(){
    dir=$(ls -l ./ |awk '/^d/ {print $NF}')
    for i in $dir; do
        cd $i
        echodir
        echo $(PWD)
        go test
        cd ..      
    done
}

echodir
echo "done"