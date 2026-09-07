allprojects {
    repositories {
        google()
        mavenCentral()
    }
}

val newBuildDir: Directory =
    rootProject.layout.buildDirectory
        .dir("../../build")
        .get()
rootProject.layout.buildDirectory.value(newBuildDir)

subprojects {
    val newSubprojectBuildDir: Directory = newBuildDir.dir(project.name)
    project.layout.buildDirectory.value(newSubprojectBuildDir)
}
subprojects {
    project.evaluationDependsOn(":app")
}

// AGP 9 sync*LibJars expects typedef recipe files; ensure stubs exist early.
subprojects {
    val recipeBase = newBuildDir.dir(project.name).asFile
    listOf("debug" to "Debug", "release" to "Release").forEach { (flavor, buildName) ->
        val recipeDir = java.io.File(
            recipeBase,
            "intermediates/annotations_typedef_file/$flavor/extract${buildName}Annotations",
        )
        recipeDir.mkdirs()
        val recipe = java.io.File(recipeDir, "typedefs.txt")
        if (!recipe.exists()) {
            recipe.writeText("")
        }
    }
}

tasks.register<Delete>("clean") {
    delete(rootProject.layout.buildDirectory)
}
