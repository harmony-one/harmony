import requests
amazon_ipv4_url = "http://169.254.169.254/latest/meta-data/public-ipv4"
def get_my_ip():
    return current_ip = requests.get(amazon_ipv4_url).text

if __name__ == "__main__":
    current_ip = requests.get(amazon_ipv4_url).text
    f = open("global_nodes.txt","r")
    peerList = []
    for myline in f:
        mylist = myline.split(" ")
        ip = mylist[0]
        node = mylist[2]
        if str(ip) != str(current_ip):
            if node != "transaction":
                peerList.append(myline)
        else:
            if node == "transaction":
                h = open("isTransaction.txt","w")
                h.write("I am just a transaction generator node")
                h.close()
    f.close()
    g = open("global_peerlist.txt","w")
    for myline in peerList:
        g.write(myline)
    g.close()
