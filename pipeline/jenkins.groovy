pipeline {
    agent any

    parameters {
        choice(name: 'OS', choices: ['linux', 'darwin', 'windows'], description: 'Target OS')
        choice(name: 'ARCH', choices: ['amd64', 'arm64'], description: 'Target architecture')
        booleanParam(name: 'SKIP_LINT', defaultValue: false, description: 'Skip running linter')
        booleanParam(name: 'SKIP_TESTS', defaultValue: false, description: 'Skip running tests')
    }

    environment {
        CGO_ENABLED = '0'
        TARGETOS = "${params.OS}"
        TARGETARCH = "${params.ARCH}"
    }

    stages {
        stage('Checkout SCM') {
            steps {
                checkout scm
                sh "git config --global --add safe.directory '${env.WORKSPACE}'"
            }
        }

        stage('Set up Go') {
            steps {
                container('golang') {
                    sh '''
                        echo "Setting up Go 1.23"
                        curl -LO https://golang.org/dl/go1.23.linux-amd64.tar.gz
                        tar -C /usr/local -xzf go1.23.linux-amd64.tar.gz
                        export PATH=$PATH:/usr/local/go/bin
                        go version
                    '''
                }
            }
        }

        stage('Lint') {
            when { expression { return !params.SKIP_LINT } }
            steps {
                container('golang') {
                    sh '''
                        curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s latest
                        ./bin/golangci-lint run --timeout=5m -v -buildvcs=false
                    '''
                }
            }
        }

        stage('Test') {
            when { expression { return !params.SKIP_TESTS } }
            steps {
                container('golang') {
                    sh 'go test ./...'
                }
            }
        }

        stage('Build') {
            steps {
                container('golang') {
                    sh "make build TARGETOS=${TARGETOS} TARGETARCH=${TARGETARCH} -ldflags '-X=github.com/ARmrCode/kbot/cmd.appVersion=-' "
                }
            }
        }

        stage('Docker Build & Push') {
            steps {
                script {
                    VERSION = sh(
                        script: 'git describe --tags --abbrev=0 2>/dev/null || echo v0.0.0-$(git rev-parse --short HEAD)',
                        returnStdout: true
                    ).trim()
                    echo "Using version: ${VERSION}"
                }

                withCredentials([string(credentialsId: 'GHCR_PAT', variable: 'GHCR_TOKEN')]) {
                    container('golang') {
                        sh '''
                            echo $GHCR_TOKEN | docker login ghcr.io -u $USER --password-stdin
                            make image push TARGETOS=${TARGETOS} TARGETARCH=${TARGETARCH}
                            yq -i ".image.tag = \\"${VERSION}\\" | .image.arch = \\"${TARGETARCH}\\"" helm/values.yaml
                            git config user.name jenkins
                            git config user.email jenkins@local
                            git add helm/values.yaml
                            git commit -m "Update Helm image tag to ${VERSION}" || echo "No changes to commit"
                            git push https://${GHCR_TOKEN}@github.com/ARmrCode/kbot.git HEAD:main
                        '''
                    }
                }

                container('golang') {
                    sh "make clean TARGETARCH=${TARGETARCH}"
                }
            }
        }
    }
}
