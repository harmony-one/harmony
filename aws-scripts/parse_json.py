import json

def get_public_ip(all_reservations):
	all_public_ip_addresses = []
	for individual_instances in all_reservations:
		instance_information = individual_instances['Instances'][0]
		if "running" != instance_information["State"]["Name"]:
			continue
		all_public_ip_addresses.append(instance_information['PublicIpAddress'])
	return all_public_ip_addresses

def make_peers_list(all_reservations,port="9001",filename="config.txt"):
	p = get_public_ip(all_reservations)
	f = open(filename,"w")
	for i in range(len(p)):
		if i == 0:
			f.write(p[i] + " " + port + " " + "leader"+"\n")
		else:
			f.write(p[i] + " " + port + " " + "validator"+"\n")
	f.close()

def is_it_running(f):
	pass

if __name__ == "__main__":
	json_data=open("aws.json").read()
	f = json.loads(json_data)
	all_reservations = f['Reservations']
	
	make_peers_list(all_reservations)
	
			
			
		