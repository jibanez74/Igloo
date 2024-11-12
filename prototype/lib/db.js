import mongoose from "mongoose";
import { createClient } from "redis";

const client = createClient({ url: process.env.REDIS_HOST });
client.connect(); // Connect to Redisimport { createClient } from "redis";

const exec = mongoose.Query.prototype.exec;

mongoose.Query.prototype.cache = function (options = {}) {
  this.useCache = true;
  this.hashKey = JSON.stringify(options.key || "");

  return this;
};

mongoose.Query.prototype.exec = async function () {
  if (!this.useCache) {
    return exec.apply(this, arguments);
  }

  const key = JSON.stringify(
    Object.assign({}, this.getQuery(), {
      collection: this.mongooseCollection.name,
    })
  );

  const cacheValue = await client.hGet(this.hashKey, key);

  if (cacheValue) {
    const doc = JSON.parse(cacheValue);

    return Array.isArray(doc)
      ? doc.map(d => new this.model(d))
      : new this.model(doc);
  }

  const result = await exec.apply(this, arguments);

  await client.hSet(this.hashKey, key, JSON.stringify(result));

  await client.expire(this.hashKey, 10); // Set expiration separately

  return result;
};

mongoose.set("strictQuery", true);

export const db = mongoose;
export const redis = client;
