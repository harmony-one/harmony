echo "Getting list of Availability Zones"                                          
all_regions=$(aws ec2 describe-regions --output text --query 'Regions[*].[RegionName]' | sort)
all_az=()                                                                          
                                                                                   
echo $all_regions
while read -r region; do                                                           
  az_per_region=$(aws ec2 describe-availability-zones --region $region --query 'AvailabilityZones[*].[ZoneName]' --output text | sort)
  echo $region $az_per_region                                                                                   
  while read -r az; do                                                             
    all_az+=($az)                                                                  
  done <<< "$az_per_region"                                                        
done <<< "$all_regions"  
echo $all_az
