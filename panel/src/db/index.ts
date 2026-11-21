import { drizzle } from "drizzle-orm/node-postgres";
import { Pool } from "pg";

const connectionString = process.env.DATABASE_URL;

if (!connectionString) {
  throw new Error("DATABASE_URL is required for the panel database");
}

const pool = new Pool({ connectionString });

export const db = drizzle(pool);
export { pool };
