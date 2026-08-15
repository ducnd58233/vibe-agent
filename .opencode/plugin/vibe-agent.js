/**
 * vibe-agent control plane for opencode.
 *
 * opencode is the one host of four that publishes no shell-command hook
 * surface. Claude Code, Cursor and Codex each read a config file naming a
 * command to run; opencode's lifecycle is reachable only from a plugin. That is
 * why this file exists and why the other three hosts do not have one.
 *
 * Registering an MCP server, which this repository already did, is not a
 * substitute. The model decides whether to call a tool, and a control plane the
 * model may skip is not a control plane. These hooks fire whether or not the
 * model cooperates, which is the entire point.
 *
 * Every call shells out to `vibe-agent hook <event> --client opencode`. Nothing
 * about run state, memory, or guards is reimplemented here: this file is an
 * adapter between opencode's hook signatures and the binary's stdin/stdout
 * contract, and it should stay small enough to read in one sitting.
 *
 * The envelope it parses is recorded in
 * .ai-agents/references/host-hook-contracts.md, generated from
 * runtime/internal/harness/contracts.go. When one side changes, that table is
 * the thing to change with it.
 */

import { spawn } from "node:child_process";

/** How long a hook may take before it is abandoned. */
const HOOK_TIMEOUT_MS = 5000;

/**
 * Run one vibe-agent hook and return whatever JSON it printed.
 *
 * Never throws and never rejects. A control plane that fails a session over its
 * own bookkeeping is worse than one that records nothing, which is the same
 * rule the Go side follows: an absent binary makes every hook a quiet no-op,
 * and that is a supported way to run this toolkit.
 *
 * @param {string} event vendor-neutral event name
 * @param {string} directory workspace root
 * @param {object} payload what the binary reads from stdin
 * @returns {Promise<object>} parsed reply, or {} when there was none
 */
async function callHook(event, directory, payload) {
  return new Promise((resolve) => {
    let child;
    try {
      child = spawn(
        "vibe-agent",
        ["hook", event, "--client", "opencode", "--workspace", directory],
        { stdio: ["pipe", "pipe", "pipe"] },
      );
    } catch {
      resolve({});
      return;
    }

    let stdout = "";
    let settled = false;
    const finish = (value) => {
      if (settled) return;
      settled = true;
      clearTimeout(timer);
      resolve(value);
    };

    const timer = setTimeout(() => {
      child.kill();
      finish({});
    }, HOOK_TIMEOUT_MS);

    child.stdout.on("data", (chunk) => {
      stdout += chunk;
    });
    // Consumed and discarded. The binary writes diagnostics here, and an
    // unread pipe fills and stalls the child.
    child.stderr.on("data", () => {});
    child.on("error", () => finish({}));
    child.on("close", () => {
      try {
        finish(stdout.trim() ? JSON.parse(stdout) : {});
      } catch {
        finish({});
      }
    });

    child.stdin.on("error", () => finish({}));
    child.stdin.end(JSON.stringify(payload));
  });
}

export const server = async ({ directory }) => {
  // Remembered so the same context is not appended to every single request.
  // opencode has no session-start hook that can inject, so the bootstrap text
  // rides along on the first system transform instead.
  let injectedSession = false;

  return {
    /**
     * The injection point, and the only one that reaches every request.
     *
     * Named experimental by opencode, so this is the part most likely to need
     * rewriting. The gate and the journal below use stable hooks and do not
     * depend on it: if this stops working, the control plane still refuses and
     * still records, and only the context goes quiet.
     */
    "experimental.chat.system.transform": async (_input, output) => {
      const event = injectedSession ? "user-prompt-submit" : "session-start";
      const reply = await callHook(event, directory, {});
      injectedSession = true;
      if (reply.additional_context) {
        output.system.push(reply.additional_context);
      }
    },

    /**
     * The journal. Records what ran, against the active run or the workspace.
     *
     * opencode reports no exit status and has no failure event, so every call
     * is recorded as a success. That is a gap in the host rather than a choice
     * here, and it is written down in the contract: it means a failed command
     * on opencode never becomes a memory, the same blind spot Codex has by a
     * different route.
     */
    "tool.execute.after": async (input, output) => {
      await callHook("post-tool-use", directory, {
        tool_name: input.tool,
        tool_input: input.args ?? {},
        tool_response: output?.output ?? "",
      });
    },

    /**
     * The refusal path.
     *
     * permission.ask rather than tool.execute.before, because throwing from
     * the latter reads to the model as a broken tool rather than as a decision
     * about one. Here the verdict is data, and the model is told why.
     *
     * Only a deny is applied. A hook that answered "allow" would be widening
     * permissions opencode had already decided to ask about, which is not this
     * gate's job: it exists to refuse things, never to approve them.
     */
    "permission.ask": async (input, output) => {
      const reply = await callHook("pre-tool-use", directory, {
        tool_name: input?.type ?? "",
        tool_input: input?.metadata ?? {},
      });
      if (reply.permission === "deny") {
        output.status = "deny";
      }
    },
  };
};
