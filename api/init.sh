#!/bin/bash

export JWT_ISSUER=igloo.ibanezcoelho.com
export JWT_AUDIENCE=igloo
export JWT_SECRET="9hA3Ks3BduhKLjGq4ShtxmVQtLY+JhI/izqh9rXQjhE"
export JWT_EXPIRATION=900
export JWT_REFRESH_EXPIRATION=2592000
export COOKIE_NAME="igloo"
export COOKIE_PATH="/"
export COOKIE_DOMAIN="localhost"

go run .
