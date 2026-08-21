---
title: Taxonomies, Ontologies, Semantic Models & Knowledge Graphs
tags:
- Artificial Intelligence
- Machine Learning
- Taxonomy
- Ontology
- Semantic Model
published: '2022-04-28'
updated: '2022-04-30'
free: true
freedium_url: https://freedium-mirror.cfd/https://medium.com/@jim.mchugh/taxonomies-ontologies-semantic-models-knowledge-graphs-5aa4d4137eba
source_url: https://medium.com/@jim.mchugh/taxonomies-ontologies-semantic-models-knowledge-graphs-5aa4d4137eba
---

# Taxonomies, Ontologies, Semantic Models & Knowledge Graphs

*Published Apr 28, 2022 · Updated Apr 30, 2022 · Free: Yes*

Several people have recently asked me about taxonomies, ontologies, and semantic models and why they are important. In this blog post, I hope to show you why these are foundational steps to Knowledge Graphs, and by extension, to AI/ML solutions.

**Taxonomies**

A taxonomy is a hierarchical framework, or schema, for the organization of organisms, inanimate objects, events, and/or concepts. We see taxonomies daily as humans, and we don't give them much thought. Taxonomies are the facets, filters, and search suggestions commonly seen on modern websites.

For example, books can be categorized as fiction and nonfiction at a high level. That may work in some instances, but in most cases, that is too high of a grouping level, so we further subdivide each high-level category until we are satisfied we have achieved an appropriate grouping level. Figure 1 shows an example of a taxonomy for books.

<picture>
  <source media="(max-width: 768px)" srcset="/img/medium/700/0*eE50nbS28mq0D9Xh.jpg 1x">
  <source media="(min-width: 769px)" srcset="/img/medium/2000/0*eE50nbS28mq0D9Xh.jpg 1x">
  <img src="/img/medium/700/0*eE50nbS28mq0D9Xh.jpg" alt="None" width="300" height="169" loading="lazy" data-zoom-src="/img/medium/4000/0*eE50nbS28mq0D9Xh.jpg" class="prose-image" data-caption="Figure 1. — Book Taxonomy"/>
</picture>

Another taxonomy example is how you sort your documents on your computer. For example, some may choose to start with a subject and then sub-divide by year, while others may do the opposite.

There are no absolute right and wrong with taxonomies, just degrees of appropriateness. The most important question to ask when creating a taxonomy is, "does this hierarchical grouping meet my needs?"

**Ontologies**

According to Wikipedia, an [ontology](https://en.wikipedia.org/wiki/Ontology_(information_science)#:~:text=In%20computer%20science%20and%20information,or%20all%20domains%20of%20discourse.) "encompasses a representation, formal naming, and definition of the categories, properties, and relations between the concepts, data, and entities that substantiate one, many, or all [domains of discourse](https://en.wikipedia.org/wiki/Domain_of_discourse)." In other words, ontologies allow us to organize the jargon of a subject area into a controlled vocabulary, thereby decreasing complexity and confusion. Without ontologies, you have no frame of reference, and understanding is lost. As Robert Engles states in his blog post [On the role and the whatabouts of Ontology](https://dutchbob.medium.com/1-on-the-role-and-the-whatabouts-of-ontology-1fb57db6ca18), <mark class="bg-emerald-300 dark:bg-emerald-700 dark:text-white">ontologies are "essential in modern architectural patterns to ensure data quality, governance, findability, interoperability, accessibility, and reusability."</mark>

For example, an ontology will allow one to associate the Book taxonomy with the Customer taxonomy via relationships.

An ontology is more challenging to create than a taxonomy because it needs to capture the interrelationships between business objects/concepts by encapsulating the language and terminology of the business area you are modeling.

<picture>
  <source media="(max-width: 768px)" srcset="/img/medium/700/0*KCXThXaJan34k9oC.png 1x">
  <source media="(min-width: 769px)" srcset="/img/medium/2000/0*KCXThXaJan34k9oC.png 1x">
  <img src="/img/medium/700/0*KCXThXaJan34k9oC.png" alt="None" width="300" height="297" loading="lazy" data-zoom-src="/img/medium/4000/0*KCXThXaJan34k9oC.png" class="prose-image" data-caption="Figure 2. — OntologiesStephen DeAngelis, [Ontology Power of Understanding](https://enterrasolutions.com/blog/ontology-power-understanding/)"/>
</picture>

A properly created ontology will expose the understanding of how the elements in the model relate to each other. Based on this understanding, one can infer intent via the relationships. A virtual assistant like Alexa uses these relationships phrases and synonyms of those phrases to define the user's intention.

**Semantic Data Model**

A Semantic Data Model is a method of organizing data that reflects the basic meaning of data items and the relationships among them. An example of a semantic model is a conceptual data model. This model has enough information to convey meaning to someone who may not know or understand the subject area.

We call **semantic models** to contain the ontology and the factual knowledge in a large, combined model with definitions added to concepts, links, and facts based on business needs.

**Knowledge Graphs**

Knowledge graphs are models that instantiate the taxonomy and ontology via a semantic model using the actual data and associated relationships. These graphs are the foundation for us to realize the promise of Artificial Intelligence (AI) and Machine Learning (ML) capabilities by capturing and exposing the relationships between nodes. These relationships contain data and metadata about the relationship between nodes, which is very different from the inferred relationships between columns of data in a relational database.

This relationship data and metadata is critical to successful Machine Learning (ML) solutions. By creating an understanding of the relationships between the nodes, we can achieve progressive improvements to the improvement of the data model without creating and injecting new code. These incremental improvements to the knowledge graph are critical to implementing Artificial Intelligence (AI) because this mimics how the human brain can reassess a concept or situation based on new data and derive a course correction.

AI/ML solutions like this already exist and are used every day. Fraud detection solutions, virtual assistant tools like Alexa, Netflix recommendations, and the "someone you may know" features on Facebook or LinkedIn use AI/ML, built on taxonomies, ontologies, and semantic data models.

<picture>
  <source media="(max-width: 768px)" srcset="/img/medium/700/0*mEYs46_694svL_AA.jpg 1x">
  <source media="(min-width: 769px)" srcset="/img/medium/2000/0*mEYs46_694svL_AA.jpg 1x">
  <img src="/img/medium/700/0*mEYs46_694svL_AA.jpg" alt="None" width="300" height="169" loading="lazy" data-zoom-src="/img/medium/4000/0*mEYs46_694svL_AA.jpg" class="prose-image" data-caption="Figure 3. — Fraud Detection Knowledge Graph"/>
</picture>

In conclusion, don't try to skip steps. Instead, start any AI/ML solution by ensuring you have laid a good foundation of understanding via taxonomies and ontologies that will create a robust and flexible knowledge graph.