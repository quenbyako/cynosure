// This plugin is a special tool that collects usage stats of opencode sessions.
// Primary goal of this plugin is analyzing common operations with agents, and
// then, optimizing pipelines, to minimize token usage (tokens are expensive).

import type { Plugin } from "@opencode-ai/plugin";
import * as fs from 'fs';
import * as path from 'path';

export default (async ({ directory }) => {
  const logPath = path.join(directory, '.opencode', 'token-usage.log');

  return {
    "tool.execute.after": async (input, output) => {
      // Log tool usage along with arguments to analyze bottlenecks
      const logEntry = {
        timestamp: new Date().toISOString(),
        tool: input.tool,
        args: input.args, // Included to analyze command-specific bottlenecks
      };

      try {
        fs.appendFileSync(logPath, JSON.stringify(logEntry) + '\n');
      } catch (err) {
        // Silently ignore to avoid disrupting operation
      }
    },
  }
}) satisfies Plugin
