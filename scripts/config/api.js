const axios = require("axios");
const keys = require("./keys");

const api = axios.create({
  baseURL: keys.API_URL,
});

module.exports = api;
