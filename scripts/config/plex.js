const axios = require("axios");

const plex = axios.create({
  baseURL: keys.PLEX_HOST,
  headers: { "X-Plex-Token": keys.PLEX_TOKEN },
});

module.exports = plex;
