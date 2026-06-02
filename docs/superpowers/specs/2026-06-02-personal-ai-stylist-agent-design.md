# Personal AI Stylist Agent Design

Date: 2026-06-02

## Product Positioning

The product is a personal AI image consultant delivered through a WeChat mini program. It helps users who have clear self-improvement and appearance goals make better decisions about clothing, hairstyle, and overall personal image.

The first version is not a general wardrobe inventory tool and not a shopping app. It focuses on a repeatable advisory loop:

1. Lightweight onboarding
2. Initial action-oriented image report
3. Daily scenario-based outfit and hairstyle suggestions
4. User feedback
5. Memory updates that improve future recommendations

Future expansion can include 7-day improvement plans, body-shape or weight-loss plans, makeup, and more complete shopping recommendations, but these are outside the first MVP.

## Target User

The target user is a normal consumer with a personal "becoming better-looking" goal. They are not only looking for a one-time outfit recommendation. They want the assistant to gradually understand their appearance, preferences, lifestyle, constraints, and feedback, then give increasingly personalized suggestions.

The MVP should prioritize continuing usage over one-time report completeness. The product succeeds when users come back because the agent feels increasingly useful and personally aware.

## MVP Approach

The chosen approach is a mixed closed loop:

- On the first day, the user completes a 5-8 minute onboarding flow.
- The system collects only the minimum useful profile, image, wardrobe, preference, and style-reference data.
- The system immediately generates an initial action-oriented image report.
- The user can then ask daily scenario questions, such as what to wear for work, dating, interviews, client meetings, travel, or social gatherings.
- Each recommendation can be accepted, rejected, or refined with a reason.
- Feedback becomes memory and changes future recommendations.

This balances first-session trust with the long-term agent value of continuous memory and evolution.

## Core User Flow

### Onboarding

The onboarding flow should take no more than 8 minutes. It collects:

- Selfie or upper-body photo
- Optional full-body or half-body photo
- Height, body-shape notes, skin-tone perception, hair volume, and hair texture
- Career and lifestyle scenarios
- Style goals
- Style dislikes and hard constraints
- 10-20 core wardrobe items, prioritizing outerwear, tops, bottoms, shoes, and bags
- Optional preferred celebrities or user-uploaded reference images

The flow should use conversation as the main interface. Visual uploads provide evidence, while chat fills in preference, goal, anxiety, budget, comfort, and context.

### Initial Action Report

After onboarding, the system generates an initial image report. The report should be action-oriented rather than theory-heavy.

It should include:

- 2-3 suitable style routes
- Hairstyle direction
- Clothing principles for the user's body, face, skin-tone, and goals
- Outfit combinations using existing wardrobe items
- Current wardrobe gaps
- Clear avoid-list items
- Celebrity-style logic that can be referenced safely
- Next best actions for the user

The report is not a final judgment. It is a living personal image profile that can update as the user gives feedback.

### Daily Scenario Advice

The user can ask for daily advice through natural language, for example:

- "Tomorrow I am meeting a client. I want to look professional but not old-fashioned."
- "I have a date tonight and want to look relaxed but intentional."
- "Can I wear this jacket with these pants?"

The agent should ask follow-up questions only when required, such as weather, formality, comfort, available items, or the feeling the user wants to express.

Each response should include:

- Recommended outfit
- Hairstyle or grooming direction
- Why it fits the user and the scenario
- Alternative option
- What not to do
- Optional wardrobe gap suggestion

### Feedback Loop

Every recommendation should create feedback targets. The user can respond with:

- Accepted
- Rejected
- Accepted with modification
- Wore it and felt good
- Wore it but felt wrong
- Received positive or negative external feedback
- A free-text reason

The system should turn feedback into memory and use it in later recommendations. Real user feedback should have higher priority than initial AI inference.

## Functional Modules

### Conversational Agent

The conversational agent is the main product surface. It handles onboarding, missing-information follow-up, report generation, daily advice, and feedback collection.

The tone should feel like a private image consultant: specific, practical, direct, and emotionally considerate. It should not overpraise the user or make unsupported claims.

### Personal Profile

The personal profile stores stable facts and evolving judgments:

- Height and body-shape information
- Face-shape tendency
- Skin-tone tendency
- Hair volume and texture
- Occupation and lifestyle scenarios
- Style goals
- Disliked styles
- Comfort constraints
- Budget tendency
- AI-inferred traits with confidence
- User-confirmed traits

The system must distinguish user-stated facts from AI inference. Inferred profile fields should be editable and should carry confidence where useful.

### Wardrobe Profile

The wardrobe profile stores the user's core clothing, shoes, bags, and accessories.

Each wardrobe item should include:

- Category
- Color
- Silhouette
- Material or thickness
- Pattern
- Formality
- Season
- Suitable scenarios
- Image
- User notes

The MVP does not require complete wardrobe inventory management. It only needs enough core items to generate useful daily suggestions.

### Celebrity Style Library

The celebrity style library is a quality-controlled reference layer. The first version should combine:

- A manually curated celebrity style library
- User-selected preferred celebrities or uploaded reference images

The library should store style logic rather than only celebrity names. Each style sample should include:

- Celebrity or reference label
- Style keywords
- Face-shape and body-proportion cues
- Skin-tone and image-atmosphere cues
- Hairstyle and grooming characteristics
- Clothing structure
- Color and silhouette patterns
- Suitable scenarios
- Transferable elements
- Non-transferable risks

The system should avoid saying "you look like this celebrity" as the core claim. It should instead say that the user can reference the structure, proportion, color logic, hairstyle direction, or styling method from a certain celebrity style route.

The MVP should not automatically crawl celebrity images from the internet. This avoids quality, copyright, likeness, and data-source problems in the first release.

### Recommendation Engine

The recommendation engine combines:

- User profile
- Wardrobe profile
- Scenario context
- Celebrity style references
- User feedback memory
- Style and body-proportion constraints

It outputs practical recommendations. A good recommendation explains what to wear, how to style hair, why it works, what to avoid, and what wardrobe gap may be limiting better results.

### Feedback Memory

Feedback memory is the core of the agent experience. It records:

- Recommendation ID
- Scenario
- Suggested style route
- Suggested wardrobe items
- User action
- Rejection or acceptance reason
- Real-world result
- External feedback
- Profile fields or style rules that should be adjusted

Later recommendations should explicitly account for important prior feedback, especially repeated rejection patterns.

## AI Workflow

The system should use structured records plus LLM reasoning and vision-assisted extraction. It should not rely on raw conversation context alone.

### Vision Extraction

For user photos, the vision model extracts structured signals such as:

- Face-shape tendency
- Skin-tone or undertone tendency
- Hair volume and texture
- Body-proportion cues
- Clothing category
- Clothing color
- Clothing silhouette
- Material impression
- Pattern
- Season
- Formality

All AI-derived observations should be represented as uncertain unless confirmed by the user.

### Profile and Preference Capture

The agent writes user statements into structured profile, preference, and constraint fields. For example, "I do not want to look too sweet or too mature" becomes a style constraint instead of remaining only in conversation history.

### Report Generation

Report generation should follow four steps:

1. Read user profile and core wardrobe data.
2. Match transferable celebrity style samples.
3. Generate 2-3 user-specific image routes.
4. Convert each route into hairstyle, outfit, wardrobe-gap, and avoid-list actions.

### Daily Advice Generation

Daily recommendations should first gather required scenario variables. Then the engine generates 2-3 practical options and records the recommendation objects for future feedback.

### Memory Update

Feedback should update either:

- Specific item preference
- Scenario rule
- Style route preference
- Hair or grooming preference
- Disliked color, silhouette, or formality
- Personal profile inference

The system should prefer observed feedback over initial similarity matching.

## MVP Scope

The MVP must include:

- WeChat mini program product experience
- Conversational onboarding
- Selfie or upper-body photo upload
- Core wardrobe upload
- User profile and preference capture
- Manually curated celebrity style library
- User-selected celebrity or reference-image style parsing
- Initial action-oriented image report
- Daily scenario outfit and hairstyle advice
- Feedback capture
- Memory-based recommendation improvement
- Wardrobe gap suggestions without specific product links

## Non-Goals

The MVP will not include:

- Full e-commerce product recommendation
- Specific SKU links
- Automatic public-image crawling for celebrity styling
- Full wardrobe inventory management
- Medical, dermatology, hair-loss, or cosmetic-surgery advice
- Weight-loss or body-transformation plans
- Guaranteed appearance improvement claims
- Public reuse of user-uploaded reference images

## Safety, Privacy, and Product Boundaries

Photos, body information, appearance preferences, and wardrobe data are sensitive personal data. The product must support user deletion of photos and profile records.

The agent should avoid shame-based language. Descriptions of body shape, skin tone, age impression, and appearance should be neutral, specific, and actionable.

AI inference should be presented as a suggestion or tendency, not as a fixed fact. Users should be able to correct it.

Celebrity-style references should be framed as styling logic, not identity comparison. The product should avoid overemphasizing physical resemblance to a celebrity.

## Success Metrics

The north-star metric is continuous usage.

Primary metrics:

- 7-day retention
- 14-day effective return frequency

An effective return means the user performs at least one meaningful action:

- Requests a daily scenario recommendation
- Adds wardrobe items
- Gives feedback
- Updates preferences
- Reviews or refreshes the image report

Secondary metrics:

- Onboarding completion rate
- Initial report satisfaction
- Daily recommendation adoption rate
- Feedback submission rate
- Recommendation revision rate
- Celebrity-reference match satisfaction
- User-initiated preference updates

Feedback submission rate is especially important because memory is the core differentiator.

## Version Roadmap

### V0 Prototype Validation

Use a manually curated celebrity style library and semi-manual analysis to test 20-50 seed users. Validate whether users find the report useful and whether daily suggestions create repeat usage.

### V1 MVP

Launch the WeChat mini program core loop:

- User profile
- Core wardrobe
- Action report
- Daily scenario advice
- Feedback memory
- Celebrity style reference
- Wardrobe gap suggestions

### V2 Active Companionship

Add:

- 7-day improvement plans
- Reminders
- Wardrobe cleanup tasks
- Style-route iteration reports
- More proactive agent behavior

### V3 Expanded Image Management

Add only after V1 usage data is strong:

- Body posture and weight-management plans
- Makeup advice
- Shopping recommendations
- More lifestyle scenarios

## MVP Acceptance Criteria

The MVP is acceptable when:

- A user can finish onboarding within 8 minutes.
- The system can generate a practical initial image report.
- The report includes style route, hairstyle direction, wardrobe combinations, wardrobe gaps, and avoid-list guidance.
- The system can produce 2-3 outfit and hairstyle suggestions for a real scenario.
- Each suggestion can be accepted, rejected, or commented on.
- The next recommendation reflects prior feedback.
- Celebrity style references explain what to reference and what not to copy.
- The system avoids direct identity comparison claims such as "you look like this celebrity."
- Users can delete photos and profile records.
