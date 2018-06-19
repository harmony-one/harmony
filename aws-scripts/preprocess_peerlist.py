from collections import namedtuple
import requests
amazon_ipv4_url = "http://169.254.169.254/latest/meta-data/public-ipv4"
if __name__ == "__main__":
    current_ip = requests.get(amazon_ipv4_url).text
    f = open("all_peerlist.txt",r)
    for myline in f:
        mylist = myline.strip().split(" ")
        ip = mylist[0]
        if ip != current_ip:
            peerList.append(myline)
    f.close()
    g = open("global_peerlist.txt")
    for myline peerList:
        g.write(myline + "\n")
    g.close()    

    
    