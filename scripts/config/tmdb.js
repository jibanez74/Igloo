const axios = require("axios");
const keys = require("./keys");

const tmdb = axios.create({
  baseURL: keys.TMDB_API_URL,
  params: {
    api_key: keys.TMDB_API_KEY,
  },
});

module.exports = tmdb;
