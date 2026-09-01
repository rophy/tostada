domain: ${DOMAIN}

oidcMock:
  issuer: "https://${DOMAIN}"
  redirectURIs:
    - "https://${DOMAIN}/api/auth/callback"

guacamole:
  jsonSecretKey: "${GUACAMOLE_JSON_SECRET_KEY}"
