const api = require("../config/api");

async function saveMoods(moods) {
  const moodList = [];

  if (!Array.isArray(moods)) {
    const res = await api.post("/music-mood", { tag: "unknown" });

    moodList = [res.data.data];

    return moodList;
  }

  for (const m of moods) {
    try {
      const res = await api.post("/music-mood", { tag: m.tag });

      moodList.push(res.data.data);
    } catch (err) {
      console.error(err);
      process.exit(3);
    }
  }

  return moodList;
}

module.exports = saveMoods;
