import { Router } from 'express';
import * as handler from './handler.js';

/**
 * All /api/v1/* traffic is proxied to the Go billing service.
 * When you need feature-specific behavior (e.g. only invoices),
 * split routes here and keep the rest on the catch-all proxy.
 */
const router = Router();

router.use(handler.handleProxy());

export default router;