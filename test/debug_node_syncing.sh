mkdir db-127.0.0.1-9013
./bin/harmony \
  -port 9013 \
  -min_peers 3 \
  -db_dir db-127.0.0.1-9013 \
  -network_type localnet \
  -blspass file:.hmy/blspass.txt \
  -dns=false \
  -blskey_file .hmy/63f479f249c59f0486fda8caa2ffb247209489dae009dfde6144ff38c370230963d360dffd318cfb26c213320e89a512.key \
  2>&1 | tee -a ./tmp_log/log_127.0.0.1-9013.txt &

mkdir db-127.0.0.1-9110
./bin/harmony \
  -port 9110 \
  -min_peers 3 \
  -db_dir db-127.0.0.1-9110 \
  -network_type localnet \
  -blspass file:.hmy/blspass.txt \
  -dns=false \
  -blskey_file .hmy/16513c487a6bb76f37219f3c2927a4f281f9dd3fd6ed2e3a64e500de6545cf391dd973cc228d24f9bd01efe94912e714.key \
  2>&1 | tee -a ./tmp_log/log_127.0.0.1-9010.txt &
