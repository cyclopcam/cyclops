sudo groupadd --system cyclops

sudo useradd --system \
 --gid cyclops \
 --create-home \
 --home-dir /var/lib/cyclops \
 --shell /usr/sbin/nologin \
 --comment "Cyclops Camera Security System" \
 cyclops
