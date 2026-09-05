# ApplyForge — Product Overview

## Vision

ApplyForge is an AI-powered operating system for the job search: job discovery, deterministic job matching,
AI-assisted resume tailoring, rapid skill preparation, application tracking, and analytics.

Primary promise: go from a newly posted job to an optimized, application-ready resume in minutes.
Secondary promise: explain exactly what a candidate needs to learn to discuss unfamiliar technologies in an interview.

## Core Workflow

```
Discover → Match → Tailor → Learn → Apply → Prepare → Track
```

## Product Philosophy

* AI proposes changes; the user makes the final decision. Nothing requires course/quiz/project completion before approval.
* The system never fabricates employers, dates, titles, degrees, certifications, metrics, clearances, compensation, or projects.
* AI-introduced claims (skills/bullets not present on the master resume) are always visibly disclosed before approval.
* Immigration compatibility (H-1B transfer, new H-1B sponsorship, green card/PERM history) is treated as a first-class,
  separately-explained signal — never collapsed into a single "sponsorship: yes/no" boolean, and never silently baked
  into the technical match score.

## Positioning

Tagline: **Go from job posting to interview-ready.**
Not "another AI resume builder" — positioned as the operating system for the entire job search.

## Navigation (MVP)

Dashboard, Jobs, Resume, Applications, Learning, Analytics, Profile, Settings.

## MVP Definition of Success

A user can: create an account, upload a master resume, configure target jobs, see recently discovered/fresh jobs,
see a deterministic Job Match Score and understand why they match, see transferable and missing skills, tailor
their resume with AI, approve suggestions immediately, use Quick Prep and Defend This Bullet, download PDF/DOCX,
apply via the official link, track the application, and see basic analytics.

## Status

This document describes the target product. As of Phase 0, only repository scaffolding, docs, and health-check
skeletons for the three services exist — no product features are implemented yet. See
[IMPLEMENTATION_PLAN.md](IMPLEMENTATION_PLAN.md) for phase sequencing and [DECISIONS.md](DECISIONS.md) for what
was actually decided/built so far.
