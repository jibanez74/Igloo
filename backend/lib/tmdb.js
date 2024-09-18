import axios from "axios";

const tmdb = axios.create({
  baseURL: process.env.TMDB_API_URL,
  params: {
    api_key: process.env.TMDB_API_KEY,
    append_to_response: "credits",
  },
});

export default tmdb;
