import axios from "axios";

const client = "NodeClient";
const clientVersion = "1.0.0";
const clientName = "NodeClient";
const username = process.env.JELLYFIN_USER;
const password = process.env.JELLYFIN_PASSWORD;

const jellyfin = axios.create({
  baseURL: process.env.JELLYFIN_API_URL,
  headers: {
    "Content-Type": "application/json",
    "X-Emby-Authorization": `MediaBrowser Client="${clientName}", Device="NodeClient", DeviceId="12345", Version="${clientVersion}"`,
    "X-Emby-Token": process.env.JELLYFIN_TOKEN,
  },
});

export default jellyfin;
