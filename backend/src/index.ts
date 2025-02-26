import { Hono } from 'hono';
import { sql } from "bun";

const app = new Hono();

app.get("/", async c => {
  const movies = await sql`
  SELECT id, title, thumb, year FROM movies ORDER_BY created_at DESC LIMIT 12 
  `;

  return c.json({
    movies,
  }, 200)
})

export default app;
