#!/bin/bash

function expense
{
   local step=$1
   local duration=$SECONDS
   logging $step took $(( $duration / 60 )) minutes and $(( $duration % 60 )) seconds
}

function logging
{
   echo $(date) : $@
   SECONDS=0
}

logging balances checking

bin/wallet balances --address one1y0xcf40fg65n2ehm8fx5vda4thrkymhpg45ecj
bin/wallet balances --address one18lp2w7ghhuajdpzl8zqeddza97u92wtkfcwpjk
bin/wallet balances --address one1tqa46jj9ut8zu20jm3kqv3f5fwkeq964t496mx
bin/wallet balances --address one19y2r8ykaztka3z8ndea0a2afd5kgswyfeahsmf
bin/wallet balances --address one16jvl43d059a4wpderqu82wlm7t3qzw8yta3wgn
bin/wallet balances --address one1kyyt7j29h4uhtnuhfar5wmngntx4gterrkd8q9
bin/wallet balances --address one10ap00fxkdupc0tnh5gvaqapw3fcxyvw2d22tlx
bin/wallet balances --address one129s9f828f538jrjca2wwlwphsl5k8rlzjdeacq
bin/wallet balances --address one19jtrujyvqdvdn9wm9tne5d4vrwvy36y69msczs
bin/wallet balances --address one180mfv4dneefp9g74duxhspjvkmcjffstd0sj6q
bin/wallet balances --address one1nvu626slwt6hwq2cup846nepcl2apuhk38gl3j
bin/wallet balances --address one16y3pzva57c65wwfpwr7ve63q67aztedsphv069
bin/wallet balances --address one14uzsrvucmxx5wwkx46r9a6mpqgtjlrchelw5pp
bin/wallet balances --address one1pmcysk3kctszln8n89hzcrgmncnqcdxg6nl2gg
bin/wallet balances --address one12z5xslc3654gxs6eh3x5seanhv976dnhvuetsu
bin/wallet balances --address one13hrrej58t6kn3k24fwuhzudy7x9tayh8p73cq9
bin/wallet balances --address one1c4w9danpa5v9zqurnl07lkqdcwyn3yfm86anqu
bin/wallet balances --address one17nfgz8rtgpl3nlws5q9tdk9y3puyqf847az6ne
bin/wallet balances --address one16f3f9y4sqtrk3eq7gnagr4ac8p25rf08u0pxxp
bin/wallet balances --address one1s7s40ku4ms63066h34xwmm5j5k4jwk74gml5nx
bin/wallet balances --address one1gqdnwl6zmn9avnaksqv2555x388nr792v7gzjr
bin/wallet balances --address one1zgmd5s6fyv9rm2vuf3augqf3ucnp9a2j0h09u3
bin/wallet balances --address one10dw0xnkm6qvpmsmeszw5wn29et9jmek9sc6dmw
bin/wallet balances --address one1ldthk5zyrea6w60rptcfyzlftd8fsn9fhkeps7
bin/wallet balances --address one17nacqwrnwgq7pk8eehn7j6jphxt0draqpkztpf
bin/wallet balances --address one1sedghemtml7xad54fdvglvch29ajv85qe2td4l
bin/wallet balances --address one1mk6g87mtgcyy95xp0v87q0srggmusp950gpn3w
bin/wallet balances --address one16295hjtqyr0z22swaqthv7mvmvn2gltnj5gera
bin/wallet balances --address one1q563tnpv4tnh7l30p2wy3gnu3akhd6va97w7ku
bin/wallet balances --address one1y7fs65ul4zc33d2502ql6nxs7r7jj4grs5x3y9
bin/wallet balances --address one149aw0kne2qwyxkxhz9v0msgf00lndvvdjne4rq
bin/wallet balances --address one1grt0frrmy7a8239sg3tygh83nd5q74yymq2ljh
bin/wallet balances --address one1zzhwus03x3j3fgtust0v07k7rf583rrp84zdet
bin/wallet balances --address one1sp687xe0kk93ngp8kaxa2qd8yjm56wjmup8mf5
bin/wallet balances --address one1ksqcladc3r5s90v494h9tfwdhkx88tq6j549f6
bin/wallet balances --address one1a37yjkrxdzwkkl3ntkqsv305grcklpksxkwsrq
bin/wallet balances --address one1c0aau6shrtxqgenpf5ymtrspxpvw0sxj0c7hrq
bin/wallet balances --address one1w2m2al52exugww4c2nn0fl2gqx3lfvhsq3x4k3
bin/wallet balances --address one1zvaqqafg0nvmxtsvzkqalc3lz378qw5j6ysz5l
bin/wallet balances --address one1z39jl5tgz3e3ra6fkru4wdnyvakrx0323zyr8v
bin/wallet balances --address one1sgcpjc405ueglhp5udsskjxcn8crrc2lmuf35c
bin/wallet balances --address one1teymhzlyuxv73hw78gy7vlfuyv3e4stvsmer5l
bin/wallet balances --address one1dsgmswzkspx3at5gywltd97sj45lapaqmk0hzw
bin/wallet balances --address one17y8k8adagmzc6t54xrnl3jmtgvmdqh2wzexn3x
bin/wallet balances --address one108uwrdejhf3es7rn6h4cdjqnvnpv75pp7t5ne9
bin/wallet balances --address one1u33urreh2uquc562geg34q374l2clqammt6fpr
bin/wallet balances --address one12zelq8ax3k48tfzl5zz37ndknremq6um62dwxa
bin/wallet balances --address one1df4tldae3amrkyrf96tg9pqccjvkjetattl4w8
bin/wallet balances --address one1yeay879a7dln5ltnchytx8eennpz332qn7yjx3
bin/wallet balances --address one1tqwwn2rh58fjafysl9rgpxjgjz8wdjmqgdwlv3
bin/wallet balances --address one15n4k4d7cw5wyyf3pt3fnwuvcweuxmmq0knpnyh
bin/wallet balances --address one18ky073zdrrmme3fs7h63wyzguuj6a3uukuc3gk
bin/wallet balances --address one1a4nhuqsa7d2znx8yq7tsuyf86v6tuq597y925l
bin/wallet balances --address one12tthayx2u7g262afmcfca29ktnx9aajjd8uj5j
bin/wallet balances --address one1c6m2w8t0p3de3cjleu2t2duvspas6366jtf8da
bin/wallet balances --address one1y5686zfh8vnygxglrhztahh7hcn2tvk33vsgrt
bin/wallet balances --address one15u2v6f56pj3rzvwge4dwl3ylg5zh3395nzj57y
bin/wallet balances --address one1s3typcymaa5dgvfu68jw0hufl7vyu0hd3hscku
bin/wallet balances --address one1qcecvkv9w77rfz75t0s7x8xpgtw0nwve2vk2sv
bin/wallet balances --address one14tdlgysvnqcdgwnduttd0y5pp2y7m8cpss30j4
bin/wallet balances --address one12vyznqd6lz6wwr9gkvd6q5zy9sswx792dh2eyv
bin/wallet balances --address one1fdtcrkpkhm2ppnu05zmddgqvledqh7g6r2tgdy
bin/wallet balances --address one1s4rypls26kmzg03dxkklmpwhmv8u4nlh6vqkdv
bin/wallet balances --address one1uhqaf9jgeczmuxs7ydzfeevwnt63ftps752cnr
bin/wallet balances --address one1khuc8sclm8lr09e0r64kf3jjt684leggzp22h4
bin/wallet balances --address one1ee39d33k3ns8wpjae6kdm46620m0v2djhacas0
bin/wallet balances --address one1yqu97zy04zy0cu6mr2gddvs94d4j2zums7ttvt
bin/wallet balances --address one1lhyk86r4a2v7gd8yhq2m0k9l2pk64y3z75zx8r
bin/wallet balances --address one1xhwspfzgv3vh5fp9hxwngv8tvdj2qr338lmavw
bin/wallet balances --address one19verfm5jyu9ys6s4nrzm6a8888kzdlvmqpenh4
bin/wallet balances --address one19qy96szsrhuyjfrqgr4gzhaaw8cgct7ym83wy3
bin/wallet balances --address one17tka7fdf9s95c597e2petfrqtnylcksvnuelzz
bin/wallet balances --address one1juqumez0qr2jwacj7vvvf79t2pnmnr24nw3cec
bin/wallet balances --address one12lecjc8a3sk35hyc5dg7te2q2cakt24d6lj2p4
bin/wallet balances --address one1780wg58e86rs38we6ze2ts930s0qmmu40vmzya
bin/wallet balances --address one1r9hjnk6zmnkageyvvsypcw2p675x7qrurjeaan
bin/wallet balances --address one1mgwlvj9uq365vvndqh0nwrkqac7cgep2pcn6zl
bin/wallet balances --address one1xsf70cu7uuu5k6f0kpxp9at8r4dmg0sttzx40t
bin/wallet balances --address one1df49l0afjgltkaheussp8e7y708ac9zuyfpfle
bin/wallet balances --address one1kq0xzzzlrpkzslwfesrgmp5e7umuxl3m3dgk27
bin/wallet balances --address one16m5r7awa4y2z2cyage4cns4uejxx8rn0gw77ug
bin/wallet balances --address one1ha85rtgc4u96v4v9nwam5qhchswx8d579dw0sl
bin/wallet balances --address one10hzlc82dhc35nz75srutrhqkk7vvvyjnewclt7
bin/wallet balances --address one10z5d98vpm5pvzw32vpma3p70vcdk0ckq0znapk
bin/wallet balances --address one12saruhnv9f63dqhuadjq3vhqm3nwyw2ac40uyz
bin/wallet balances --address one1zfare9m39n3m4h5sj6rpfdkatzheww3zs8ctwg
bin/wallet balances --address one1cxc5j0ygyrkq4lsvln3amdu3ys239jqzk56ykc
bin/wallet balances --address one1mr3mt2ra8mwpr55uv3ymv0lmdy2s0w4m5nt0jh
bin/wallet balances --address one1gemlvpun4528ajv7pcn2d9fufzcv80kjt3dxwg
bin/wallet balances --address one1pnjl29uv2avuv5ts9nejwecc5em37yu92vllqn
bin/wallet balances --address one1nhu8np4ztt3f5xpxt4rznshhc7amm5p7fsf3xe
bin/wallet balances --address one1lg5nes6j2gy5tkhexhc0zs98uay9ddj05tdpq9
bin/wallet balances --address one1mzlm2tt9uhas0qnk86nwxqfuhvjnh5349pf23z
bin/wallet balances --address one1ymudu3v8f8uv3gvzr7p6n0z0wyvsqkls6mhef0
bin/wallet balances --address one1nek27yzpawmqyw5lpm5tshcruz3c08sum5eke3
bin/wallet balances --address one1trghegqlxyvkq8suavy6tmhfppcdg4rng3xlct
bin/wallet balances --address one1mna6y63xwzh5zaajsj42trrn9tgkqcwa2gaszs
bin/wallet balances --address one1madlfrpp4t7z8mgk86smfqnvcssfvclzszscg0
bin/wallet balances --address one1r3mh2h7flr6sgcjvpaadlfjcnguwfk5z6mjuvu
bin/wallet balances --address one1528xlrfresl7wl2nr37kp0tnlr7hr3w8u5y4dp
bin/wallet balances --address one1284fqe06amt67j6cwrj6c2yrcjudeq0uygjyq3
bin/wallet balances --address one1ughluft6keduyy8v8nhczkyfmcc2qhe9wxpmrz
bin/wallet balances --address one1yshyk4y59na2lzscgpmw4jqgtjpl33vrpnukep
bin/wallet balances --address one160wta3dr4jt85d70ct7psuumduf8yayysnq5j3
bin/wallet balances --address one1y469t49t34c7sylrd9pthfam6xmlz5p3zh2zdj
bin/wallet balances --address one1vme5xkhn8mtff325jz0frcy8k5zru88zz75gn5
bin/wallet balances --address one1c2rtqwsqs65cput3q9yaejp3dxjwct207we9hm
bin/wallet balances --address one12l30mhevl4q3wqde2jm28fp8jqxqmjhxrn5asq
bin/wallet balances --address one17ffl9csu7ln3jw07fcvlmh2h8e5pv3ddk9p5sv
bin/wallet balances --address one1flv4r3udp08az7axdcz9me50kr2r4z65c8s39m
bin/wallet balances --address one198azjqrgmnj3nzrau8r67n8c74ggrh7nys0p88
bin/wallet balances --address one1tl3alzt8a82n86uzeedrjn8tmxwh9mf3xmw04t
bin/wallet balances --address one1nlptlw8srthgljachm4w5rgv8ulvkt3cgk4uqq
bin/wallet balances --address one1dmh3frumlx4xrwymfdx5g7an8nrm4jjc0msgfm
bin/wallet balances --address one135mn6c90n0kd4247cramqxgeqqx5hvp506g3vd
bin/wallet balances --address one14ajehwyxpzpzxhke77mhtt0z6k5z6cevgf6rfa
bin/wallet balances --address one1hxqhp9tls9r4v5hz208g93exhvz5ak258ut7d2
bin/wallet balances --address one1wt5darzj8wd385xl8stccj4sv6553hgckaypfr
bin/wallet balances --address one19rcnp7l258uevu2h8vcraklt7uw38l4w0ll88z
bin/wallet balances --address one1l7m0e50v0fus6e4wp29fprppj9dyxekhr9qaak
bin/wallet balances --address one10vy4gdzga08vhenqke36x67fjqukyzk6d4hrqw
bin/wallet balances --address one1dzdr2vjddwxam73m7hnmy63ruuzd6qgqerfrgu
bin/wallet balances --address one1vhqp3g7epjzvemr2w6rc4xglen53vwlnkdpgzy
bin/wallet balances --address one12zjkjpj0zp3lq4k3hnld5w0t5f933pntdjw0z9
bin/wallet balances --address one1efat5elqnvttf7gm86q9kmt48z69njax464rhv
bin/wallet balances --address one1jqzm93lcjyyxdg8tad53gksly4ym5fvx3zpkw2
bin/wallet balances --address one16zed62vdgszcg0gp69zw009xsw03pg3h6uudlf
bin/wallet balances --address one18683c2vyr4xdv4wd3ley8wd250pnmxn346s4qq
bin/wallet balances --address one1sntqyfuyk4s4kxze7h5sws02jnka287v47q4de
bin/wallet balances --address one1hrdt5e5lepygmj2vfthjzauuc9085lpnfjhha4
bin/wallet balances --address one1hrg76d5743k5x8jmyu4zyn232fzdexf06w32s3
bin/wallet balances --address one12kdc0fqxne5f3394wrwadxq9yfaquyxgtaed3q
bin/wallet balances --address one1h2dynptqmtgdfg9fgpd8dvmv8scupkgtzapx4l
bin/wallet balances --address one176nrt06rjmg9qunhf0z7wdhphdjtpq94y73d7s
bin/wallet balances --address one1qndvm7y956s00ewfmtt6vf5qpzl7f3pa3q4lsa
bin/wallet balances --address one1ahw70lq9sqqygs8f7zdrvw7zd796w5n48hc5xh
bin/wallet balances --address one1g2h035trcm7dshsq9nqxzacr08ane0gx7kwjwp
bin/wallet balances --address one15qcw9x6q43r95q8udef5p2rkzu8hdkhljvxjww
bin/wallet balances --address one1cx5dtllm52wrur463t04szxqc6mpjdn0u8h4qt
bin/wallet balances --address one1va04plqqjd33ekyrnu9gfx2e3c2emu5zznut04
bin/wallet balances --address one1p3u89p0p4nxaj5sdcx0s20g5u9a3xjccjmfwuu
bin/wallet balances --address one1ljvq9tkvfp583zzl85mgjh3qjvjufnuwmn7krv
bin/wallet balances --address one12c23ekslj469g0g0tu9jcvecfkla7rahmrhe37
bin/wallet balances --address one19l9equxmql4jkcah8g4f6qva732npajarffj6q
bin/wallet balances --address one1nq5dglmw0vunsa34mve8sdyrkhfd0373v4xgtv
bin/wallet balances --address one1qmgqawpflw4pu9ytryz69mrk0mhhsswdmjgfrj
bin/wallet balances --address one1h7c7pgwnht4nns40k6swdzwy8xn9uvl0e65e49
bin/wallet balances --address one1cwzleselrsq3x76vjzy7u65a9tqmsrcne2w83h
bin/wallet balances --address one1kkcw2y5d9w9celf0vu025hflyxu33gekmntx9u
bin/wallet balances --address one1n3gzyslx97aw6fccdxk0msatnxkk2yhtwytwsa
bin/wallet balances --address one175jcxcdk2xlmccndr2mux3c8se8gsmddesg5ed
bin/wallet balances --address one1lmqycl6wezcdf7nqxj34slstamt0hlhp4s0rj4

expense balances
