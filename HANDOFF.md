# Reffector handoff — accompany *Simplicity Is Security*, Part IV

**Prepared:** 2026-10-18  
**Status:** planning and editorial handoff; no repository changes authorized or made  
**Inputs inspected:** `reffector.zip` (October 16 working tree with Git history), `code-complexity.zip` (private reference only), `fewer-places-to-go-wrong.md`, and the supplied drafts of *Simplicity Is Security* Parts II–IV.  
**Companion draft:** `when-machines-write-the-code-DRAFT.md`; its complete text is embedded in Appendix A below.

## 1. Why Reffector matters to the essay

Part IV of *Simplicity Is Security* examines the **new boundary between a developer and software written by a probabilistic coding agent**. The developer remains responsible for what the program does and what it is allowed to do, even if the agent chose much of the implementation. There are two distinct security boundaries:

1. **The agent's working environment:** what the agent can read, modify, execute, or reach while writing code.
2. **The deployed application's environment:** what privileges and capabilities the agent-generated program possesses when it runs.

A sandbox around the agent does not prove that the program it writes has appropriate permissions. Functionality passing tests doesn't prove the absence of unwanted behavior. Saltzer and Schroeder's discussion of the negative requirement to prevent unauthorized behavior makes this distinction explicit. OpenBSD's audit tradition, combined with its adoption of privilege separation and `pledge`/`unveil`, demonstrates why inspection and enforced containment are complementary.

The proposed engineering sequence is:

1. **Specify and build:** establish intended behavior *and permitted authority*, implement, and collect credible functional-test evidence.
2. **Simplify:** use the agent for bounded, incremental reductions in complexity; protect behavior with suitable tests after each change.
3. **Describe:** ask the agent what the code actually does, not simply whether it is secure; inventory effects, reachable call paths, and granted privileges with evidence and uncertainty.
4. **Constrain and verify:** reconcile requirements, implemented effects, and granted privileges; remove unnecessary authority; test both permitted and denied behavior.

Reffector is an **optional evidence source in step 3**, and can inform effect-boundary improvements in step 2. It is not the methodology, a comprehensive security scanner, or a substitute for examining deployment identities and policies.

## 2. Current Reffector implementation — confirmed from archive

Repository: <https://github.com/Version2beta/reffector>.

- CLI: `go install github.com/Version2beta/reffector/cmd/reffector@latest` (after an appropriate public version is published); `reffector ./...`, `reffector -json ./...`, `reffector -direct ./...`, `reffector -tests -json <package>`.
- Analyzer is read-only with respect to analyzed source; it uses Go package loading and SSA summaries to propagate recognized operations through calls, methods, closures, generics, recursion, and some resolved interface dispatch.
- Current effect vocabulary: `fs.read`, `fs.write`, `fs.create`, `fs.remove`, `net.connect`, `net.listen`, `process.exec`, `process.exit`, `env.read`, `env.write`, `db.query`, and `db.exec`.
- JSON records contain `package`, `function`, `effects` (`identifier`, `direct`), and `uncertain`.
- `direct: false` means an effect was inherited through calls, not that the function has been proved harmless. `uncertain: true` marks unanalyzed reachability; uncertainty is **not** an effect. `uncertain: false` does not establish full coverage or purity.
- Standard-library summaries and other heuristics are intentionally limited. Generic `io.Reader`/`io.Writer` and unknown interface calls require source inspection; unresolved receivers are not expanded to all implementations.
- **Not implemented:** shared-memory mutation modeling; exact filesystem paths, network destinations, credentials, permissions, AWS/IAM or operating-system privilege discovery, reflective/dynamic coverage guarantees, all build variants, or a judgment that an operation is justified.
- Installable agent skill: `reffector skill install` provides the **read-only `find` workflow**. `rework` and `refactor` are not implemented; neither CLI nor skill modifies inspected application source.
- Existing tests include CLI integration fixtures for propagation, closures, generics, interfaces, uncertainty, flags, workspaces, and source-tree preservation; skill-install tests also exist. The bundled man page and README are substantial.
- Go requirement is **1.26+** according to `go.mod` and README.
- The archived Git tree has no version tags; `reffector-self.json` and `.DS_Store` appear untracked in the archive. Treat the JSON as a local analysis artifact, not as proof that CI has run.
- **Validation not performed here:** the available compiler is `go1.23.2`, below the repository's declared Go requirement. Do not claim `go test`, `go vet`, or a third-party end-to-end analysis passed.

**Assessment:** The read-only analyzer plus `find` skill is close to being a useful initial public tool, subject to validation on a supported Go version and a complete realistic demo. A resource-specific, deployment-aware privilege auditor would be a much larger and separate feature.

## 3. Release plan — bounded work, in priority order

### P0 — release blockers for a truthful initial tool

**P0.1: Verify the current repository as it is.**

- Use Go 1.26+ in a clean environment. Record exact `go version` and commit SHA.
- Run `go build ./...`, `go test ./...`, and `go vet ./...` from the module root; preserve exit codes and logs.
- Exercise the packaged install path for both binary and skill (in an isolated temporary home). Confirm the original repository remains unchanged by analysis and that re-installation will not overwrite an unrelated or locally modified skill.
- Inspect generated/temp artifacts, dependency-lock behavior, and repository status. Do not silently delete `.DS_Store` or `reffector-self.json`: establish ownership first.
- On failure, fix the smallest demonstrated defect and add a regression test. Preserve existing user work; no unrelated changes.

**P0.2: Publish a reproducible example grounded in actual output.**

- Select a small, nontrivial Go package with an effect mixed into otherwise reusable computation. The prior discussion identified `os.Exit` in CLI command handling as a possible candidate, but verify actual source before presenting it as fact.
- Show the command, original report, relevant source locations, behavior that must be preserved, and a *proposed* smaller effect boundary.
- Include an example where an effect is **appropriately located** and no refactor is recommended. This prevents the tool appearing to optimize for purity or fewer reported effects.
- Explicitly distinguish direct from inherited findings and show at least one `uncertain: true` example; no inference of purity from an empty report.
- If showing a before/after refactor, perform that change only in an authorized example repository and demonstrate the preserved contract with tests. An analyzer-output change is not itself correctness evidence.

**P0.3: Align docs and install story with the implementation.**

- Review README, bundled `SKILL.md`, man page, CLI `-h`, and examples for consistent terminology and supported options.
- Explain installation prerequisites (Go 1.26+) and the difference between CLI analysis and optional read-only skill.
- Make explicit that the analyzer identifies **recognized possible effects**, not dynamically exercised operations, deployment permissions, security vulnerabilities, or every shared-memory write.
- Document the behavior of unmodeled interface calls, reflection, cgo/assembly, callbacks, and build tags at the point where users interpret results.
- Use exact outputs from P0.2 instead of hand-authored plausible-looking JSON findings.

**P0.4: Establish a public release.**

- Choose and document an initial version and compatibility expectations; tag/publish only with explicit authorization.
- Confirm install command with the published version, from a fresh Go environment.
- Publish a sparse tool page (see §5) with a working install command, concise example, limitations, and source link.

**Initial-release acceptance:** A reader with the documented Go toolchain can install and run the analyzer, interpret direct/inherited effects and uncertainty correctly, and use `find` to get no more than a few well-evidenced, *advisory* refactor opportunities. Claims are backed by reproducible commands and tests. Nothing implies a general security guarantee.

### P1 — meaningful follow-ons, not prerequisites for the essay

**P1.1: Improve audit evidence and provenance without breaking the output contract.**

- Evaluate whether source locations and a trace to the originating summarized call would make a particular reported effect easier to verify. Current records provide function names and effect kinds, not detailed provenance.
- Prototype an optional, versioned machine-readable field or separate reporting flag; don't silently alter existing JSON consumers.
- Add tests for deterministic ordering, recursive paths, closures, unresolved interfaces, and the correct propagation of uncertainty.

**P1.2: Provide a distinct capability-review workflow, without asserting unavailable detection.**

- Consider a documented prompt-based recipe that combines source analysis with *separately obtained* deployment policy, runtime identity, network/file permission evidence, and requirements.
- Outputs should show three columns: **required capability**, **implemented operation and evidence**, **granted authority and evidence**. Include an **unresolved** category.
- Do **not** call `fs.write` equivalent to permission to write an arbitrary path, `net.connect` equivalent to permission for all destinations, or `process.exec` equivalent to a deployed privilege. These categories are not resource-specific.
- Keep recommendations advisory; tools cannot infer required authority solely from existing implementation.

**P1.3: Expand detection only with a measured use case.**

- Shared-memory mutation and ownership are important to the separate *Fewer Places to Go Wrong* essay, but are outside the current analyzer model. Treat them as a prospective research/design project, not an easy addition or a release blocker.
- Consider caller-provided buffers, mutable globals, goroutines, and interface-based I/O; determine useful precision and false-positive behavior before committing to a taxonomy.
- If implemented, explicitly separate mutation/reasoning signals from actual operating-system or cloud privileges.

**P1.4: Optional controlled refactoring workflows.**

- `rework` and `refactor` are not currently present. Do not advertise them.
- If developed later, require one bounded, test-protected change per pass, clear authorization, preservation of public behavior, and a separate report of remaining uncertainty. Never equate reduced effects in JSON with increased security.

## 4. Editorial handoff — changes required to Part IV

**Full draft:** Appendix A (also delivered as `when-machines-write-the-code-DRAFT.md`). It replaces the earlier streaming-monitor draft; do not merge the old example back into it.

### Must preserve

1. Center the **new boundary between engineer and generated code**: probabilistic production of durable program behavior, without claiming every execution result is deterministic.
2. Explain **both security boundaries**: the coding agent's execution environment, and the eventual program's deployed authority. Sandboxing the former does not secure the latter.
3. Restore **infinite negative space** via Saltzer and Schroeder, not Parnas; present tests as evidence for *some* specified behavior, not proof that unspecified behavior is impossible.
4. Present OpenBSD's **auditing plus containment** as evolution/expansion, not abandonment of audits. Part III already supplies the examples of OpenSSH privilege separation and `pledge`/`unveil`; avoid repeating their detailed history.
5. Make **simplification a separate, substantive phase**, never just preparation for better auditing. Large generated diffs change the scale of reasoning. Show multiple focused, bounded prompts; do not reproduce, name as a product, or distribute the private `/code-complexity` skill.
6. Demonstrate **excellent behavioral and regression testing after each change**, without confusing preservation of observed behavior with validation against the specification.
7. Center the formulation **“Ask what the code does, not whether it's secure.”** Inventory effects and privileges with source evidence, unknowns, and requirements that are independent of the implementation. Reffector is useful here but optional and limited.
8. End by admitting that neither this workflow nor the four essays is a full secure-development methodology. Their subject is how simpler designs can also be more secure. Emphasize that **working code begins the simplification and security work rather than completing it**.

### Editorial checks before publishing

- Compare the Part I treatment of de Raadt and Saltzer/Schroeder with the new draft; do not attribute a verbatim “does only what it is specified to do” quotation to de Raadt without confirming the exact primary source. The draft currently paraphrases Part I.
- Compare terms with Parts II and III: unnecessary behavior, interface obligations, privilege separation, containment, authority. Avoid repeated explanations, and link to those essays where appropriate.
- Confirm that the hypothetical TLS example uses `InsecureSkipVerify` **without adequate custom verification**; Go supports safe custom-verifier uses. A successful TLS exchange alone does not establish peer authentication.
- The draft's phrase “simplicity is itself a security property” is a rhetorical position. If too categorical, prefer “simplicity is a security objective in its own right.” Empirical reduction in vulnerability counts is not established by a shorter implementation.
- Retain the distinction between code effects and **granted authority**. Do not claim Reffector can verify runtime IAM, file paths, shared-memory ownership, or security compliance.
- Keep agent priority claims behavioral: task completion and passing tests are common immediate feedback; don't imply the model has a fixed internal goal or no learned security knowledge.
- Verify intra-series URLs, footnote URLs, publication metadata, and whether Reffector actually has a usable public release at time of publication.
- Keep an essay independent of any unpublished tool. If Reffector is not released, use an accurate source link and label it as a developing tool, or defer that one paragraph; no reason to delay the whole series.

## 5. Tool page — sparse background, useful operational detail

**Working heading:** `Reffector — find effect boundaries in Go`.

Proposed page content, after P0 validation:

- **One-sentence description:** A read-only Go static analyzer that reports recognized side effects reachable from functions and highlights uncertainty. Its optional agent skill proposes small, evidence-backed improvements to effect boundaries.
- **One genuine command and output excerpt:** e.g. `reffector -json ./...` with a verified record, and plain-language distinction between direct/inherited and uncertain.
- **How to install:** verified Go toolchain prerequisite, binary installation, optional `reffector skill install`, and one example `$reffector find ./...`.
- **When it helps:** investigating unexpected filesystem/network/process/database operations; locating code that mixes decisions with effects; considering smaller, testable responsibilities.
- **What it doesn't do:** report every memory mutation, list actual resources/permissions, detect arbitrary vulnerabilities, prove purity, or automatically refactor code.
- **Links:** README/repository, supported workflow documentation, *Fewer Places to Go Wrong* (general side-effects argument), and Part IV (how a coding agent can use such evidence without treating it as a security verdict).

Do not publish `/code-complexity` or its internal rubric, scripts, scoring model, or complete prompts. Part IV's small independent prompts are intended for publication and do not require that private asset.

## 6. Operational notes and safeguards for the next implementer

- Start read-only. Establish branch, status, toolchain, and intended scope. Treat the archived checkout as evidence, not as authorization to commit, push, tag, or publish.
- Protect the existing analyzer's precision and uncertainty semantics; do not trade them for a more impressive-looking effect count.
- Prefer one demonstrated improvement per change. Test and review after each, with source preservation and explicit evidence of any impact on observable behavior.
- A production privilege audit requires an explicit deployment identity and policy context. Do not invent those details from source files or effect identifiers.
- The supplied code-complexity archive is **private reference material only**; do not include it in the repository, tool page, or published essay.

## Appendix A — full working Part IV draft

The following block is the exact current draft, embedded so this handoff is self-contained. The separate Markdown file is the convenient version for editing and publication.

~~~~markdown
---
title: "Simplicity Is Security: When Machines Write the Code"
linkTitle: "When Machines Write the Code"
url: "/writing/simplicity-is-security-4-when-machines-write-the-code/"
weight: 4
type: "essay"
author: "M. Robert Martin"
dek: "The engineer remains responsible when an agent makes the implementation decisions."
description: "How agentic coding adds a boundary between the engineer and the program, and why working code must be simplified, described, and constrained."
date: 2026-10-16
draft: true
toc: true
---

We ask a coding agent to implement a TLS client. It writes the code, builds it, runs tests, fixes failures, and reports that the client connects successfully. The change may be large enough that reading every line would take more time than generating it did.

Suppose the implementation contains a configuration option that sets Go's `tls.Config.InsecureSkipVerify` without replacing the disabled checks with appropriate custom verification. The client works. It connects to servers and exchanges encrypted data. But it doesn't establish that the server is the one we intended to reach.[^tls]

We saw this problem in [Part II](/writing/simplicity-is-security-2-less-is-more/). The difference now is who made the decision to offer the bypass. An engineer who writes the client may introduce it deliberately, carelessly, or by mistake. With an agent, the decision may arrive as one detail among thousands that we didn't individually choose.

This is the fourth and final essay in [*Simplicity Is Security*](/writing/simplicity-is-security/). The first three examined unintended behavior, unnecessary capabilities, and the limits of understanding. When machines write the code, all three problems remain. We have also inserted a new boundary between ourselves and the program.

## The boundary we added

A programmer's instructions are not the same as a program's instructions.

The agent generates code through a probabilistic process. The resulting program is a concrete implementation that will run according to the language and runtime's rules, with whatever inputs, scheduling, and environment it encounters. The choices that became source code are no longer suggestions; they are part of what we may compile and deploy.

We supplied the requirements, but another system chose much of the implementation: the dependencies, abstractions, error handling, defaults, and perhaps even the permissions requested by its deployment configuration. Its immediate feedback may be that the code builds and the tests pass. Neither simplicity nor the absence of unnecessary authority follows from that result.

We remain responsible for both of the system's security boundaries.

The first is **what the agent can do while working**. It may read files, execute commands, modify the workspace, or contact external services. Those operations need restrictions appropriate to the development task. An agent reading a repository shouldn't thereby acquire authority to carry out every instruction it encounters there.

The second is **what the program the agent produces can do when deployed**. It may read credentials, modify accounts, execute subprocesses, change configuration, or communicate over the network. Sandboxing the coding agent doesn't tell us whether the application it wrote has been granted too much authority.

This second boundary is easy to miss. We can restrict the agent's access to production perfectly well, then accept a deployment change that gives the finished application privileges its requirements never called for.

When we wrote the code ourselves, we had to ask whether we gave it more authority than it needed. Now we must also ask whether we successfully prevented the agent from doing so—and how we would know if we hadn't.

## Working code is the starting point

In Part I, we distinguished software doing what it is specified to do from software doing nothing it is not specified to do. The first is the easier part to demonstrate. We can write functional tests, exercise success and failure paths, and check that the implementation produces the required results.

Even then, passing tests is evidence about the cases exercised, not a general proof of correctness. And it says still less about the enormous number of things we never asked the program to do.

Saltzer and Schroeder described this negative quality of security in *The Protection of Information in Computer Systems*. Preventing all unauthorized use is difficult to establish because we would have to anticipate every possible threat. They also noted that unwanted access paths may go unnoticed in normal use—precisely because normal use doesn't attempt them.[^saltzer]

A TLS client that successfully retrieves a page hasn't demonstrated that it rejects an impostor server. A passing test that checks a transfer completed hasn't demonstrated that an unrelated caller cannot alter another account's balance. The behavior we specified occupies a small part of what the program might be able to do.

OpenBSD's history suggests a better response than ever larger audits. Its developers have long examined code closely, searching not only for known vulnerabilities but for ordinary defects that might later prove exploitable. That work continues. But they also introduced privilege separation and restrictions such as `pledge` and `unveil`, which limit what a process can do even when mistakes escape inspection.[^openbsd][^pledge]

The lesson isn't that auditing has become useless. It's that inspection and containment answer different questions. An audit asks what defects we can find. An enforced boundary limits what an undiscovered defect can accomplish.

With agent-generated code, we should carry that distinction forward. First make the implementation manageable. Then investigate its actual behavior and authority. Finally, enforce the limits the problem requires.

## Simplicity at agent scale

A pull request containing tens of thousands of lines of human-written code would ordinarily demand an exceptional justification. An agent can produce changes of that scale much more easily. The cost of generating code has fallen; the effort required to understand its interactions hasn't fallen in the same proportion.

This is why simplicity matters more, not less, when agents write software. The agent's initial implementation may work while carrying duplicated decisions, unnecessary indirection, unclear ownership, sprawling error paths, and state changes scattered through unrelated functions. Those are not merely aesthetic problems. They enlarge the amount of behavior we must account for when establishing correctness and security.

We shouldn't respond with another unbounded request: *Simplify this entire codebase.* That asks the agent to make another large collection of decisions, this time while altering code that already works.

Instead, ask it to find one bounded simplification supported by evidence. For example:

> Identify one place where the same rule or decision is implemented more than once. Show the callers, the required behavior, and the tests that would detect a regression. Propose the smallest consolidation; don't edit yet.

Or:

> Identify state whose ownership is unclear or whose mutation requires understanding several unrelated functions. Trace the readers and writers. Suggest one change that would make ownership explicit without changing behavior.

Or:

> Find a function that mixes a calculation with filesystem, network, database, or process operations. Explain whether separating those responsibilities would reduce dependencies or make testing more direct. Keep the effects somewhere appropriate; don't merely move them to another file.

These are different investigations, not three ways of saying “make it shorter.” A useful simplification removes an obligation or dependency. It may require adding code, as OpenSSH's privilege separation did in [Part III](/writing/simplicity-is-security-3-the-limits-of-understanding/). It may also require leaving well-structured code alone.

Each proposed change needs its own contract and evidence. Before making it, establish the behavior to preserve. Run the relevant tests. Make one bounded change. Run those tests again, inspect the diff, and examine changed error paths, concurrency, resource lifetimes, and public contracts. If a test fails or the intended behavior is uncertain, stop and investigate. Don't roll five refactorings into a single change whose regression is difficult to locate.

Tests deserve special attention here. Characterization tests can help us avoid accidentally changing existing behavior, but they don't prove that the existing behavior was authorized in the first place. That distinction must remain visible throughout the work.

The point isn't to make generated code pleasant to review. **Simplicity is itself a security property worth pursuing:** fewer unnecessary interactions, clearer ownership, and smaller mechanisms whose correctness and authority we can establish with less uncertainty.

## Ask what the code does, not whether it's secure

Once the implementation is working and has been simplified in bounded, tested passes, we still have to confront its negative space.

Asking an agent, “Audit this code for security vulnerabilities,” can produce useful findings. But it also invites a broad judgment about an enormous space of possible failures. That judgment may be supported by little more than the same code we were worried about trusting.

A narrower question is more useful: **What does this program actually do?**

Start with the implemented effects:

> For this package, identify functions that modify shared state or perform filesystem, network, database, environment, and process operations. Distinguish operations performed directly from those reachable through calls. Show source locations and call paths. Report interfaces, reflection, callbacks, or other paths you cannot resolve. Do not conclude that an unreported operation is impossible.

That produces a map we can inspect. It also makes questionable combinations visible. Why does a parser read a configuration file? Why can reusable library code terminate the process? Why do five packages independently modify the same record?

Go developers can use [Reffector](https://github.com/Version2beta/reffector) to help with part of this investigation. It reports recognized external effects reachable from functions and distinguishes direct from inherited effects. Its JSON output marks unresolved behavior separately. It does not establish purity, analyze shared-memory mutation, resolve resource destinations, or determine which privileges an application should possess. Those remain additional questions, not guarantees supplied by the tool.[^reffector]

Next examine the authority the program receives:

> Identify the runtime identities, credentials, sandbox restrictions, filesystem permissions, deployment policies, and privileged service operations associated with this application. For each permission, identify the source configuration and the behavior that appears to require it. Mark permissions with no established requirement. Do not change them yet.

This step isn't redundant with tracing code. An application might never call a dangerous operation in its current implementation while its service account still has permission to do so. Conversely, a call may be present but prohibited by the environment.

Finally, compare the evidence with the intended responsibility:

> Using the specification as the authority, compare required behavior, implemented effects, and granted privileges. Identify unsupported or excessive capabilities, any required operations that would be denied, and each unresolved assumption. Propose the narrowest enforceable changes. Do not treat existing code or successful tests as proof that a privilege is required.

That last instruction matters. If we derive the specification from whatever the agent happened to write, the investigation cannot tell us whether the agent added something we never wanted.

An inventory isn't an exhaustive proof. Dynamic calls, generated code, deployment differences, and imperfect analysis can leave gaps. But a report of functions, resources, permissions, and unresolved paths is more useful than an unsupported declaration that the program is secure.

## Constrain and verify

Now we have three descriptions of the same application: what it **needs** to do, what its implementation **may** do, and what its environment **allows** it to do. They need not match automatically.

Suppose the TLS client still offers an option to skip peer verification. Removing that option requires testing both halves of the contract: valid server certificates and hostnames must still work, and untrusted certificates or incorrect hostnames must be rejected. This was the point of LibreSSL's simpler TLS interface in Part II: the ordinary path should not depend on every caller remembering an independent security step.

For other software, the appropriate boundary may be an operating-system restriction, a narrowly scoped database identity, a constrained transaction interface, or a separate privileged process. The mechanism should match the problem. Narrowing a cloud permission does not automatically repair a shared-memory race; reducing side-effect sites does not substitute for authorization checks.

Ask the agent to propose one restriction at a time, including the evidence supporting it and how to verify it. Required operations must still succeed. Prohibited operations must fail in an environment representative of deployment. Tests and simulations should be described by what they actually establish, not by the confidence of the agent's explanation.

This is not a final step performed once before release. A subsequent agent-generated feature can add another dependency, effect, permission, or bypass. The same questions apply to that change too.

## Less to trust

The procedure is straightforward to state: specify and build; simplify in bounded, tested changes; describe the program's effects and authority; constrain and verify. Doing each step well takes engineering judgment. None of them guarantees that arbitrary software is secure.

Nor was that the promise of this series. *Simplicity Is Security* has examined the ways that making software simpler can also make it more secure: removing unnecessary capabilities, designing interfaces that eliminate recurring mistakes, and enforcing boundaries that limit the consequences of defects. It is not a complete method for developing secure software.

Agentic coding makes those ideas more urgent because it changes the boundary between an engineer and a program. A probabilistic agent can make thousands of implementation decisions on our behalf, producing code that may execute for years. It has no reliable, independent commitment to the simplicity or authority limits we intended merely because it has satisfied functional tests. If those requirements matter, we have to establish them explicitly and check the result.

The working implementation is where that work begins, not where it ends. We can use agents not only to produce code but to reduce the complexity they produced, document the behavior they implemented, and help us establish restrictions that remain effective when their judgment—or ours—is incomplete.

We are still responsible for the code on the other side of the new boundary. We should insist that the code be small enough to reason about where possible, constrained where understanding will always be incomplete, and supported by evidence rather than assurances.

**When machines make the implementation decisions, simplicity becomes something we must explicitly demand.**

---

[^tls]: Go standard-library documentation, [`crypto/tls.Config`](https://pkg.go.dev/crypto/tls#Config), particularly `InsecureSkipVerify`, `ServerName`, and custom verification callbacks. The example assumes disabling normal verification without an adequate replacement. It is illustrative, not a reported coding-agent incident.

[^saltzer]: Jerome H. Saltzer and Michael D. Schroeder, [*The Protection of Information in Computer Systems*](https://www.cs.virginia.edu/~evans/cs551/saltzer/), *Proceedings of the IEEE* 63(9), 1975. See Section I.A.1 for the negative requirement and Section I.A.3 for economy of mechanism and fail-safe defaults.

[^openbsd]: OpenBSD, [*Security — Audit Process and New Technologies*](https://www.openbsd.org/security.html). The project describes auditing as continuing work alongside exploit mitigations, privilege separation, and other containment mechanisms. This is an expansion of its security strategy, not an abandonment of auditing.

[^pledge]: OpenBSD, [`pledge(2)`](https://man.openbsd.org/pledge.2) and [`unveil(2)`](https://man.openbsd.org/unveil.2). These restrict classes of operations and filesystem paths respectively; they do not establish correctness of permitted operations.

[^reffector]: [Reffector source and README](https://github.com/Version2beta/reffector). The current analyzer reports recognized categories of external operations and transitive reachability, with an `uncertain` flag; its documented limits include shared-memory mutation, destinations, permissions, plugins, and build-configuration completeness.

~~~~
