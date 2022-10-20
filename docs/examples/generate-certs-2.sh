#! /bin/bash

DOMAIN="localhost.local"

HOSTS="server client"

SUBJ_BASE="/C=DE/ST=SH/L=E/O=MariaDB/OU=Support/emailAddress=nothing@example.com"

CA_SUBJ="$SUBJ_BASE/CN=$DOMAIN"

openssl genrsa 2048 > ca-key.pem

openssl req -sha256 -new -x509 -nodes -days 3600 -key ca-key.pem -out ca-cert.pem -subj $CA_SUBJ


for HOST in $HOSTS
do
	SUBJ="$SUBJ_BASE/CN=$HOST.$DOMAIN"

	# Create server certificate, remove passphrase, and sign it
	# Create the server key
  openssl req -sha256 -newkey rsa:2048 -days 3600 -nodes -keyout $HOST-key.pem -out $HOST-req.pem -subj $SUBJ

	# Process the server RSA key
	openssl rsa -in $HOST-key.pem -out $HOST-key.pem

	# Sign the server certificate
  openssl x509 -sha256 -req -in $HOST-req.pem -days 3600 -CA ca-cert.pem -CAkey ca-key.pem -set_serial 01 -out $HOST-cert.pem

	# Verify certificates
	openssl verify -CAfile ca-cert.pem $HOST-cert.pem
done


