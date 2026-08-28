______________________________________________________________________

title: End Toil by Doing Nothing. But Better. Perpetually.
published: false
description: Dissolving Toil using a Do Nothing Script to get to Self-Driving, Unattended Reconciliation
tags: software, engineer, go
# published_at: 2025-03-01 17:59 +0000

______________________________________________________________________

In 25 years, I've worn pretty much all the hats in the software industry.
A superpower that I got from my time in SRE was to recognize and eliminate my own _personal_ [toil](https://sre.google/sre-book/eliminating-toil/).

The [Google SRE book defines organizational Toil](https://sre.google/sre-book/eliminating-toil/#toil-defined) as Manual, Repetitive, Automatable, Tactical, of No enduring value, and O(n) with service growth.

Ignoring the focus on the _organization_ or the _service_, how much of the work _you_ are asked to do sounds like any of that?

For me, I found that most non-SRE roles people spend _most_ of their time on toil.

There is a reliable, general process for automating toil so you can focus on the more engaging aspects of most work. It's worth noting that this is nothing special to an SRE, even if it seems shocking or subversive to non-SRE people.

Any specific example might not be relevant to you (e.g., Timesheet autofill/submit, password rotation, etc.).

If you've got access to an LLM, you can even use your company's Clanker to follow this process to get out from whatever BS you were asked to toil away at.

1. Start with a Dan Slimmon [Do-Nothing script](https://blog.danslimmon.com/2019/07/15/do-nothing-scripting-the-key-to-gradual-automation/):

    > A do-nothing script is a script that encodes the _instructions_ of a slog, encapsulating each step in a function.
    >
    > This script doesn’t actually do any of the steps of the procedure. That’s why it’s called a do-nothing script. It feeds the user a step at a time and waits for them to complete each step manually.
    >
    > At first glance, it might not be obvious that this script provides value. Maybe it looks like all we’ve done is make the instructions harder to read. But the value of a do-nothing script is immense:
    >
    > It’s now much less likely that you’ll lose your place and skip a step. This makes it easier to maintain focus and power through the slog.
    >
    > Each step of the procedure is now encapsulated in a function, which makes it possible to replace the text in any given step with code that acts automatically.
    >
    > Over time, you’ll develop a library of useful steps, which will make future automation tasks more efficient.
    >
    > A do-nothing script doesn’t save your team any manual effort. It lowers the activation energy for automating tasks, which allows the team to eliminate toil over time.

    Dan Slimmon even [has a nice little Go framework for "Do Nothing Scripting"](https://github.com/danslimmon/donothing). 
  
2. Create a Run Book for your Job (or Life) and Maintain it
  
    In ancient times, a Run Book (or system operation manual) was written by the IT operations (Ops) team after software development was considered complete.
  
    These days, whenever manual intervention is required, that intervention's process is documented (in some wiki or CMS). Every time someone has to repeat that, they attach a note to that.
  
    If a process is at all recurrent, start with a "Do Nothing Script". Similarly, if the risk of human error is high, it's also a good idea to start with a "Do Nothing Script". If it's a one-off, low-risk process, probably forget about it. The investment you spend in both creating and improving that script should be proportional to the frequency and the stakes of the process.

3. Evolve your Do-Nothing Script into an Interactive-Do-Something Task Runner in your Ubiquitous Language.

    Any single Step that seems like too much work for too little value can still pause, prompt you with the manual instruction, and remain manual so it will "Do Nothing".

    For everything else, scratch what itches most (whatever bothers you most) and automate low-hanging fruit (easiest to automate).

    It's extremely important for your task runner and its task definitions to be written in the same ubiquitous language (e.g., Go) that you use for your day-to-day development. You feel the pain most acutely, so you need to be empowered to improve your own situation, instead of having a barrier of learning an unfamiliar syntax before beginning a speculative attempt to improve your situation.

    Another benefit is that since it is just a programming language you already know (e.g., Go), you have all the benefits of static analysis, access to the entire ecosystem of libraries, and the ability to write testable, modular code.

    You can also open-source it (or pieces of it) and collaborate with others to mutually benefit!

4. Evolve to "Fire and Forget" with "Log and Notify."

    Automate interactive steps to be fully non-interactive by adding Observability

    Whenever something switches to being _unattended_, you are no longer babysitting it and watching unexpected errors scroll by.

    If things go sideways, you aren't there to hit Ctrl-C to immediately intervene with full context.

    You need it to keep some kind of durable record of what exactly happened, and some way of getting your attention that it may have gone off the rails, and what you should look at.

5. Hook up Event triggers to automatically trigger your "Fire and Forget."

    You can usually run a cron job that just periodically polls with "can I run now? How about now?", but it's better to find the actual event and run off of that.

6. Take it _out_ of your Run Book once it's fully self-driving

    It's no longer a repeated process. This feels so good, right?

### Further Exploration

If this seems all very basic to you, then some further readings that might be useful are here.

#### Theory of Constraints

The [Theory of Constraints](https://en.wikipedia.org/wiki/Theory_of_constraints) is based on the premise that the rate of goal achievement by a goal-oriented system (i.e., the system's throughput) is limited by at least one constraint.

The argument by reductio ad absurdum is as follows: If nothing was preventing a system from achieving higher throughput (i.e., more goal units in a unit of time), its throughput would be infinite, which is impossible in a real-life system.

Only by increasing flow through the constraint can overall throughput be increased.

Assuming the goal of a system has been articulated and its measurements defined, the steps are:

1. Identify the system's constraint(s).
2. Decide how to exploit the system's constraint(s).
3. Subordinate everything else to the above decision.
4. Elevate the system's constraint(s).
5. Warning! If in the previous steps a constraint has been broken, go back to step 1, but do not allow inertia to cause a system's constraint.

Apply this to yourself and your Job. What is your Job's Goal? What is your constraint?

I’ve also found a good Stanford lecture that has a good rundown on how to apply it to LLMs.
https://andybromberg.com/toc-ai-stanford

#### Reconciliation

Resilient Automation is already a thing, and you are unlikely to encounter a lot of the problems that require the complexity of large-scale solutions.

They're still pretty interesting!

From [The Principle of Reconciliation](https://www.chainguard.dev/unchained/the-principle-of-reconciliation):

> - At the heart of resilient systems lies a deceptively simple feedback loop: observe reality, compare it against an ideal, and act to align the two. This principle, reconciliation, is what makes systems self-healing, adaptable, and robust in the face of change and failure.
> 
> ###### Desired vs. Actual: A Systemic Tension
> 
> Every resilient system embodies a tension: the difference between the desired state and the actual state. This delta isn’t just a bug; it’s the whole point. Reconciliation exists because divergence is inevitable. But more importantly, because it’s recoverable.
> 
> ###### What is a Reconciler?
> 
> A **reconciler** implements a continuous feedback loop that makes > systems self-healing and resilient. Adapted from Kubernetes controllers, the pattern is deceptively simple:
> 
> - **Watch:** Observe the current state of the world
> - **Compare:** Compute the delta between the desired and actual state
> - **Act:** Make changes to close the gap
> - **Repeat:** Forever
> 
> ###### Level-Based vs Event-Driven
> 
> Traditional event-driven systems are fragile: lost events mean lost actions, crashed services leave work incomplete, and state drifts over time. They're edge-triggered, reacting to moments of change without remembering the desired end state.
> 
> Reconciliation systems are level-based. The workqueue holds keys, not events—multiple events about the same resource collapse into a single reconciliation of its current state. Events are just hints to check the state; the reconciler compares the actual to the desired and takes idempotent actions that are safe to repeat. This makes the system naturally resilient to event storms, duplicate notifications, and processing delays.

###### Even Further reading:

- [Desired State Systems](https://branislavjenco.github.io/desired-state-systems/)
- [Reactive planning and reconciliation in Go](https://blog.gopheracademy.com/reactive-planning-go/)
- [The Principle of Reconciliation](https://www.chainguard.dev/unchained/the-principle-of-reconciliation)
